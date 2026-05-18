import "server-only";
import { ethers } from "ethers";
import { web3Db } from "@/lib/web3-db";

type PaymentInput = {
  chain_slug: string;
  tx_hash: string;
  log_index?: number;
  from_address: string;
  to_address: string;
  token_contract: string;
  token_symbol: string;
  token_decimals: number;
  amount: string;
  raw_amount?: string;
  block_number?: number;
  raw_event?: Record<string, unknown>;
};

export async function createAccountingEntry(invoiceId: string | null, eventId: string | null, entryType: string, payload: unknown) {
  await web3Db.query(
    `insert into web3_accounting_entries (invoice_id, payment_event_id, entry_type, payload)
     values ($1,$2,$3,$4)`,
    [invoiceId, eventId, entryType, JSON.stringify(payload ?? {})]
  );
}

export async function processPaymentEvent(input: PaymentInput) {
  const chain = await web3Db.chains.bySlug(input.chain_slug);
  if (!chain) throw new Error("Invalid chain_slug");

  const eventInsert = await web3Db.query<{ id: string }>(
    `insert into web3_payment_events (chain_id, tx_hash, log_index, from_address, to_address, token_contract, token_symbol, token_decimals, amount, raw_amount, block_number, raw_event, status)
     values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'detected')
     returning id::text`,
    [chain.id, input.tx_hash, input.log_index ?? null, input.from_address, input.to_address, input.token_contract, input.token_symbol, input.token_decimals, input.amount, input.raw_amount ?? null, input.block_number ?? null, JSON.stringify(input.raw_event ?? {})]
  );
  const insertedEvent = eventInsert.rows[0];
  if (!insertedEvent) throw new Error("Failed to create payment event");
  const eventId = insertedEvent.id;

  const invoiceMatch = await web3Db.query<{ id: string }>(
    `select id::text from web3_invoices
     where chain_id = $1
       and lower(receiver_address) = lower($2)
       and lower(stablecoin_contract) = lower($3)
       and expected_amount <= $4::numeric
       and status in ('pending','partially_paid')
     order by created_at asc limit 1`,
    [chain.id, input.to_address, input.token_contract, input.amount]
  );

  await createAccountingEntry(null, eventId, "payment_detected", input);

  const invoice = invoiceMatch.rows[0];
  if (!invoice) return { eventId, matched: false };

  await web3Db.query(`update web3_invoices set status = 'paid', paid_at = now() where id = $1`, [invoice.id]);
  await web3Db.query(`update web3_payment_events set status = 'matched', invoice_id = $1 where id = $2`, [invoice.id, eventId]);
  await createAccountingEntry(invoice.id, eventId, "payment_matched", input);
  await createAccountingEntry(invoice.id, eventId, "invoice_paid", { paid_at: new Date().toISOString() });

  return { eventId, matched: true, invoiceId: invoice.id };
}

export function parseTransferLog(log: ethers.Log, tokenDecimals: number) {
  const transferTopic = ethers.id("Transfer(address,address,uint256)");
  if (log.topics[0] !== transferTopic) return null;

  if (!log.topics[1] || !log.topics[2]) return null;
  const from = ethers.getAddress(`0x${log.topics[1].slice(26)}`);
  const to = ethers.getAddress(`0x${log.topics[2].slice(26)}`);
  const raw = BigInt(log.data);

  return {
    from,
    to,
    rawAmount: raw.toString(),
    amount: ethers.formatUnits(raw, tokenDecimals)
  };
}
