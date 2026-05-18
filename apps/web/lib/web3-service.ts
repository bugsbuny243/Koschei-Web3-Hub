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

export async function createAccountingEntry(input: {
  invoice_id: string | null;
  payment_event_id: string | null;
  entry_type: string;
  amount: string;
  currency: string;
  description: string;
  metadata?: Record<string, unknown>;
}) {
  await web3Db.query(
    `insert into web3_accounting_entries (invoice_id, payment_event_id, entry_type, amount, currency, description, metadata)
     values ($1,$2,$3,$4,$5,$6,$7)`,
    [
      input.invoice_id,
      input.payment_event_id,
      input.entry_type,
      input.amount,
      input.currency,
      input.description,
      JSON.stringify(input.metadata ?? {})
    ]
  );
}

export async function processPaymentEvent(input: PaymentInput) {
  const chain = await web3Db.chains.bySlug(input.chain_slug);
  if (!chain) throw new Error("Invalid chain_slug");

  const eventInsert = await web3Db.query<{ id: string }>(
    `insert into web3_payment_events (chain_slug, tx_hash, log_index, from_address, to_address, token_contract, token_symbol, token_decimals, amount, raw_amount, block_number, raw_event, status)
     values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'detected')
     returning id::text`,
    [input.chain_slug, input.tx_hash, input.log_index ?? null, input.from_address, input.to_address, input.token_contract, input.token_symbol, input.token_decimals, input.amount, input.raw_amount ?? null, input.block_number ?? null, JSON.stringify(input.raw_event ?? {})]
  );
  const insertedEvent = eventInsert.rows[0];
  if (!insertedEvent) throw new Error("Failed to create payment event");
  const eventId = insertedEvent.id;

  const invoiceMatch = await web3Db.query<{ id: string; currency: string; expected_amount: string }>(
    `select id::text, currency, expected_amount::text from web3_invoices
     where chain_slug = $1
       and lower(receiver_address) = lower($2)
       and lower(stablecoin_contract) = lower($3)
       and expected_amount <= $4::numeric
       and status in ('pending','partially_paid')
     order by created_at asc limit 1`,
    [input.chain_slug, input.to_address, input.token_contract, input.amount]
  );

  await createAccountingEntry({
    invoice_id: null,
    payment_event_id: eventId,
    entry_type: "payment_detected",
    amount: input.amount,
    currency: input.token_symbol,
    description: "Payment transfer detected",
    metadata: { chain_slug: input.chain_slug, tx_hash: input.tx_hash }
  });

  const invoice = invoiceMatch.rows[0];
  if (!invoice) return { eventId, matched: false };

  await web3Db.query(`update web3_invoices set status = 'paid', paid_at = now() where id = $1`, [invoice.id]);
  await web3Db.query(`update web3_payment_events set status = 'matched', invoice_id = $1 where id = $2`, [invoice.id, eventId]);

  await createAccountingEntry({
    invoice_id: invoice.id,
    payment_event_id: eventId,
    entry_type: "payment_matched",
    amount: input.amount,
    currency: invoice.currency,
    description: "Payment matched to invoice",
    metadata: { tx_hash: input.tx_hash }
  });
  await createAccountingEntry({
    invoice_id: invoice.id,
    payment_event_id: eventId,
    entry_type: "invoice_paid",
    amount: invoice.expected_amount,
    currency: invoice.currency,
    description: "Invoice marked as paid",
    metadata: { paid_at: new Date().toISOString(), tx_hash: input.tx_hash }
  });

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
