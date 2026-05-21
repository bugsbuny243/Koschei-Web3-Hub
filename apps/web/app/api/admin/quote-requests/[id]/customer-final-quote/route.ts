import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";

const n = (v: FormDataEntryValue | null) => (v ? Number(v) : 0);
const t = (f: FormData, k: string) => ((f.get(k) as string) || null);

export async function POST(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const u = new URL(req.url);
  if (u.searchParams.get("password") !== process.env.ADMIN_PASSWORD) return NextResponse.json({}, { status: 401 });

  const f = await req.formData();
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "DB unavailable" }, { status: 500 });

  const supplierQuote = (await pool.query("select * from supplier_ddp_quotes where quote_request_id=$1 order by created_at desc limit 1", [id])).rows[0];
  if (!supplierQuote) return NextResponse.json({ error: "Supplier DDP quote is required before customer quote" }, { status: 400 });

  const supplierCost = Number(supplierQuote.raw_ddp_cost);
  const tradepiMargin = n(f.get("tradepi_margin"));
  const escrowFeeBuffer = n(f.get("escrow_fee_buffer"));
  const bankFxRiskBuffer = n(f.get("bank_fx_risk_buffer"));

  const finalCustomerPrice = supplierCost + tradepiMargin + escrowFeeBuffer + bankFxRiskBuffer;
  if (finalCustomerPrice <= 0) return NextResponse.json({ error: "Final price must be > 0" }, { status: 400 });

  await pool.query(
    "insert into customer_final_quotes (quote_request_id,supplier_ddp_cost_internal,tradepi_margin_internal,escrow_fee_buffer_internal,bank_fx_risk_buffer_internal,final_customer_price,payment_terms_public,delivery_terms_public,quote_notes,status) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,'approved_internal')",
    [id, supplierCost, tradepiMargin, escrowFeeBuffer, bankFxRiskBuffer, finalCustomerPrice, t(f, "payment_terms_public"), t(f, "delivery_terms_public"), t(f, "quote_notes")],
  );
  await pool.query("update quote_requests set status='customer_quote_prepared' where id=$1", [id]);

  return NextResponse.redirect(new URL(`/admin/quote-requests/${id}/escrow?password=${u.searchParams.get("password")}`, req.url));
}
