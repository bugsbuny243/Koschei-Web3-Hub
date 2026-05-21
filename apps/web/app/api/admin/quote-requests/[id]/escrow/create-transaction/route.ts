import { NextRequest, NextResponse } from "next/server";
import { createEscrowTransaction } from "@/lib/escrow-client";
import { isAdminAuthed } from "@/lib/admin-auth";
import { getDbPool } from "@/lib/db";

export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const password = request.headers.get("x-admin-password") ?? request.nextUrl.searchParams.get("password");
  if (!isAdminAuthed(password)) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "DB unavailable" }, { status: 500 });

  const quote = (await pool.query(
    `select cfq.*, qr.email from customer_final_quotes cfq join quote_requests qr on qr.id=cfq.quote_request_id where cfq.quote_request_id=$1 order by cfq.created_at desc limit 1`,
    [id],
  )).rows[0];

  if (!quote) return NextResponse.json({ error: "Final customer quote not found" }, { status: 404 });

  const { payload, data } = await createEscrowTransaction({
    buyerEmail: quote.email,
    finalCustomerPrice: Number(quote.final_customer_price),
    itemTitle: "TradePi quotation",
    itemDescription: quote.quote_notes ?? "TradePi quotation",
  });

  const record = await pool.query(
    `insert into escrow_transactions (quote_request_id,escrow_transaction_id,escrow_status,final_customer_price,buyer_email,seller_email,currency,item_title,item_description,escrow_fee_payer,escrow_fee_paid_by_tradepi,payment_link,raw_create_payload,raw_response)
     values ($1,$2,$3,$4,$5,$6,$7,$8,$9,'tradepi_globall',true,$10,$11,$12) returning *`,
    [
      id,
      String(data.id ?? ""),
      data.status ?? "draft",
      quote.final_customer_price,
      quote.email,
      process.env.ESCROW_DEFAULT_SELLER_EMAIL ?? "me",
      process.env.ESCROW_DEFAULT_CURRENCY ?? "usd",
      payload.items?.[0]?.title ?? "TradePi quotation",
      payload.items?.[0]?.description ?? "TradePi quotation",
      (data.payment_url as string) ?? null,
      JSON.stringify(payload),
      JSON.stringify(data),
    ],
  );

  await pool.query("update customer_final_quotes set status='awaiting_customer_payment' where id=$1", [quote.id]);
  await pool.query("update quote_requests set status='escrow_created' where id=$1", [id]);

  return NextResponse.json({ escrow_transaction_id: record.rows[0].escrow_transaction_id, escrow_status: record.rows[0].escrow_status, payment_link: record.rows[0].payment_link });
}
