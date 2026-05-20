import { NextRequest, NextResponse } from "next/server";
import { createEscrowTransaction } from "@/lib/escrow-client";
import { isAdminAuthed } from "@/lib/admin-auth";
import { getDbPool } from "@/lib/db";

export async function POST(request: NextRequest) {
  const password = request.headers.get("x-admin-password") ?? request.nextUrl.searchParams.get("password");
  if (!isAdminAuthed(password)) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const contentType = request.headers.get("content-type") ?? "";
  const customer_quote_id = contentType.includes("application/json") ? (await request.json()).customer_quote_id : String((await request.formData()).get("customer_quote_id") ?? "");
  const pool = getDbPool(); if (!pool) return NextResponse.json({ error: "DB unavailable" }, { status: 500 });

  const q = await pool.query(`select cq.*, qr.email from customer_quotes cq join quote_requests qr on qr.id=cq.quote_request_id where cq.id=$1`, [customer_quote_id]);
  const quote = q.rows[0]; if (!quote) return NextResponse.json({ error: "Quote not found" }, { status: 404 });
  if (!["approved_internal", "accepted"].includes(quote.status)) return NextResponse.json({ error: "Quote status is not eligible" }, { status: 400 });

  const { payload, data } = await createEscrowTransaction({ buyerEmail: quote.email, finalCustomerPrice: Number(quote.final_customer_price), itemTitle: "Fine Cleaner 5X-5 quote", itemDescription: quote.quote_notes ?? "Fine Cleaner 5X-5" });
  const record = await pool.query(`insert into escrow_transactions (quote_request_id,customer_quote_id,escrow_transaction_id,escrow_status,final_customer_price,buyer_email,seller_email,currency,item_title,item_description,escrow_fee_payer,escrow_fee_paid_by_tradepi,payment_link,raw_create_payload,raw_response) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'tradepi_globall',true,$11,$12,$13) returning *`, [quote.quote_request_id, quote.id, String(data.id ?? ""), data.status ?? "draft", quote.final_customer_price, quote.email, process.env.ESCROW_DEFAULT_SELLER_EMAIL ?? "me", process.env.ESCROW_DEFAULT_CURRENCY ?? "usd", payload.items?.[0]?.title ?? "Fine Cleaner 5X-5 quote", payload.items?.[0]?.description ?? "Fine Cleaner 5X-5", (data.payment_url as string) ?? null, JSON.stringify(payload), JSON.stringify(data)]);
  await pool.query("update customer_quotes set status='accepted' where id=$1", [quote.id]).catch(()=>null);
  return NextResponse.json({ escrow_transaction_id: record.rows[0].escrow_transaction_id, escrow_status: record.rows[0].escrow_status, payment_link: record.rows[0].payment_link, raw_response_summary: { id: data.id, status: data.status } });
}
