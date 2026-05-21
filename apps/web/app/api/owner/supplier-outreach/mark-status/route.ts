import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

const allowed = new Set(["new", "reviewed", "message_ready", "sent", "replied", "interested", "rejected", "not_relevant", "blocked"]);

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const supplierLeadId = String(body.supplier_lead_id || "");
  const status = String(body.status || "");
  const pool = getDbPool();
  if (!pool || !supplierLeadId || !allowed.has(status)) return NextResponse.json({ error: "invalid payload" }, { status: 400 });
  await pool.query("update supplier_leads set status=$2,updated_at=now() where id=$1", [supplierLeadId, status]);
  await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,'status_changed',$2,'owner')", [supplierLeadId, status]);
  return NextResponse.json({ ok: true });
}
