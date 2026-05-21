import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";
import { buildOutreachMessage } from "@/lib/ai/supplier-analysis-ai";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const supplierLeadId = String(body.supplier_lead_id || "");
  const pool = getDbPool();
  if (!pool || !supplierLeadId) return NextResponse.json({ error: "missing supplier_lead_id" }, { status: 400 });
  const message = buildOutreachMessage();
  const saved = (await pool.query("insert into supplier_outreach_messages (supplier_lead_id,subject,body,status) values ($1,$2,$3,'draft') returning *", [supplierLeadId, message.subject, message.body])).rows[0];
  await pool.query("update supplier_leads set status='message_ready',updated_at=now() where id=$1", [supplierLeadId]);
  return NextResponse.json({ ok: true, message: saved });
}
