import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const supplierLeadId = String(body.supplier_lead_id || "");
  const eventType = String(body.event_type || "");
  const pool = getDbPool();
  if (!pool || !supplierLeadId || !eventType) return NextResponse.json({ error: "invalid payload" }, { status: 400 });
  await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,$2,$3,'owner')", [supplierLeadId, eventType, null]);
  return NextResponse.json({ ok: true });
}
