import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const messageId = String(body.message_id || "");
  const pool = getDbPool();
  if (!pool || !messageId) return NextResponse.json({ error: "missing message_id" }, { status: 400 });
  const updated = (await pool.query("update supplier_outreach_messages set approved_by_owner=true,updated_at=now() where id=$1 returning supplier_lead_id", [messageId])).rows[0];
  if (updated?.supplier_lead_id) await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,'message_approved',$2,'owner')", [updated.supplier_lead_id, messageId]);
  return NextResponse.json({ ok: true });
}
