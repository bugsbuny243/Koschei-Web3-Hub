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

  const rawDdpCost = n(f.get("raw_ddp_cost"));
  if (rawDdpCost <= 0) return NextResponse.json({ error: "raw_ddp_cost must be > 0" }, { status: 400 });

  await pool.query(
    "insert into supplier_ddp_quotes (quote_request_id,supplier_name,currency,raw_ddp_cost,destination,supplier_notes) values ($1,$2,$3,$4,$5,$6)",
    [id, t(f, "supplier_name"), t(f, "currency") ?? "USD", rawDdpCost, t(f, "destination"), t(f, "supplier_notes")],
  );
  await pool.query("update quote_requests set status='supplier_cost_received' where id=$1", [id]);

  return NextResponse.redirect(new URL(`/admin/quote-requests/${id}/pricing?password=${u.searchParams.get("password")}`, req.url));
}
