import { NextResponse } from "next/server";
import { createEscrowTransaction } from "@/lib/escrow-client";
import { getDbPool } from "@/lib/db";
import { appendMilestone, isOwnerRequest } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const quoteRequestId = String(body.quote_request_id || "");
  const pool = getDbPool();
  if (!pool || !quoteRequestId) return NextResponse.json({ error: "missing quote_request_id" }, { status: 400 });
  const row = (await pool.query("select cfq.final_customer_price, qr.email, qr.company_name from customer_final_quotes cfq join quote_requests qr on qr.id=cfq.quote_request_id where cfq.quote_request_id=$1 order by cfq.created_at desc limit 1", [quoteRequestId])).rows[0];
  if (!row) return NextResponse.json({ error: "no finalized quote" }, { status: 400 });
  const trx = await createEscrowTransaction({ buyerEmail: row.email, finalCustomerPrice: Number(row.final_customer_price), itemTitle: `TradePi Machinery RFQ ${quoteRequestId}`, itemDescription: "Owner-approved official quote" });
  const trxId = String(trx.data?.id ?? "");
  await pool.query("insert into escrow_transactions (quote_request_id,customer_quote_id,escrow_transaction_id,escrow_status,payment_link,fee_payer,raw_payload) values ($1,null,$2,$3,$4,$5,$6)", [quoteRequestId, trxId || null, String(trx.data?.status ?? "created"), null, process.env.ESCROW_FEE_PAYER ?? "tradepi_globall", JSON.stringify(trx.data ?? {})]);
  await appendMilestone(quoteRequestId, "escrow created");
  return NextResponse.json({ ok: true, escrow_transaction_id: trxId, escrow_status: trx.data?.status ?? "created" });
}
