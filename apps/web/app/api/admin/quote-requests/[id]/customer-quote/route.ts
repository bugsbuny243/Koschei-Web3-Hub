import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";

const n = (v: FormDataEntryValue | null) => (v ? Number(v) : 0);
const t = (f: FormData, k: string) => (f.get(k) as string) || null;

export async function POST(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const u = new URL(req.url);
  if (u.searchParams.get("password") !== process.env.ADMIN_PASSWORD) {
    return NextResponse.json({}, { status: 401 });
  }

  const f = await req.formData();
  const supplier = n(f.get("supplier_ddp_price_usd"));
  const escrow = n(f.get("escrow_fee_internal"));
  const bank = n(f.get("bank_transfer_fee_internal"));
  const op = n(f.get("operation_cost_internal"));
  const type = (f.get("commission_type") as string) || "fixed";
  const fixed = n(f.get("commission_fixed_usd"));
  const pct = n(f.get("commission_percent"));

  const commission = type === "percent" ? (supplier * pct) / 100 : fixed;
  const final = supplier + commission + escrow + bank + op;

  const p = getDbPool();
  if (p) {
    await p.query(
      "INSERT INTO customer_quotes (quote_request_id,internal_total_cost,markup_type,markup_amount,markup_percent,final_customer_price,gross_profit,gross_margin_percent,payment_terms_public,delivery_terms_public,quote_notes,valid_until,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'approved_internal')",
      [
        id,
        supplier + escrow + bank + op,
        type,
        fixed,
        pct,
        final,
        commission,
        final ? (commission / final) * 100 : 0,
        t(f, "payment_terms_public"),
        t(f, "delivery_terms_public"),
        t(f, "quote_notes"),
        t(f, "valid_until"),
      ],
    );

    await p.query(
      "INSERT INTO tradepi_commissions (quote_request_id,commission_type,commission_percent,commission_fixed_usd,escrow_fee_internal_usd,bank_fee_internal_usd,operation_cost_internal_usd,final_customer_price_usd,commission_amount_usd,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'approved_internal')",
      [id, type, pct, fixed, escrow, bank, op, final, commission],
    );

    await p.query("UPDATE quote_requests SET status='customer_quote_prepared' WHERE id=$1", [id]);
  }

  return NextResponse.redirect(
    new URL(`/admin/quote-requests/${id}/escrow?password=${u.searchParams.get("password")}`, req.url),
  );
}
