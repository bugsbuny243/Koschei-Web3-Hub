import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";

const n = (v: FormDataEntryValue | null) => (v ? Number(v) : 0);
const t = (f: FormData, k: string) => ((f.get(k) as string) || null);

export async function POST(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const u = new URL(req.url);
  if (u.searchParams.get("password") !== process.env.ADMIN_PASSWORD) return NextResponse.json({}, { status: 401 });

  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "DB unavailable" }, { status: 500 });

  const escrow = (await pool.query("select * from escrow_transactions where quote_request_id=$1 order by created_at desc limit 1", [id])).rows[0];
  if (!escrow || !["funded", "completed", "payment_secured"].includes(String(escrow.escrow_status))) {
    return NextResponse.json({ error: "Customer escrow payment must be secured first" }, { status: 400 });
  }

  const f = await req.formData();
  const supplierCost = n(f.get("supplier_total_amount"));
  if (supplierCost <= 0) return NextResponse.json({ error: "supplier_total_amount must be > 0" }, { status: 400 });

  const depositAmount = Number((supplierCost * 0.3).toFixed(2));
  const balanceAmount = Number((supplierCost * 0.7).toFixed(2));

  await pool.query(
    `insert into supplier_payment_milestones (quote_request_id,milestone_name,milestone_percent,milestone_amount,status,private_notes)
     values ($1,'production_deposit',30,$2,$3,$4),($1,'before_shipment_or_delivery_release',70,$5,$6,$7)`,
    [id, depositAmount, t(f, "deposit_status") ?? "pending", t(f, "deposit_private_notes"), balanceAmount, t(f, "balance_status") ?? "pending", t(f, "balance_private_notes")],
  );

  return NextResponse.redirect(new URL(`/admin/quote-requests/${id}/supplier-payments?password=${u.searchParams.get("password")}`, req.url));
}
