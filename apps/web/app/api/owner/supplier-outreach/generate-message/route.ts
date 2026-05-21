import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";
import { buildOutreachMessage } from "@/lib/ai/supplier-analysis-ai";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const supplierLeadId = String(body.supplier_lead_id || "");
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "Database is not configured." }, { status: 500 });
  if (!supplierLeadId) return NextResponse.json({ error: "missing supplier_lead_id" }, { status: 400 });

  const lead = (await pool.query("select * from supplier_leads where id=$1", [supplierLeadId])).rows[0];
  if (!lead) return NextResponse.json({ error: "lead/source not found" }, { status: 404 });

  const message = buildOutreachMessage({
    productCategory: Array.isArray(lead.product_categories) ? lead.product_categories[0] : null,
    platform: lead.platform,
    companyName: lead.company_name || lead.possible_company_name || null,
    sourceUrl: lead.source_url,
  });
  const saved = (await pool.query("insert into supplier_outreach_messages (supplier_lead_id,subject,body,status) values ($1,$2,$3,'draft') returning *", [supplierLeadId, message.subject, message.body])).rows[0];
  await pool.query("update supplier_leads set status='message_ready',updated_at=now() where id=$1", [supplierLeadId]);
  await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,'message_generated',$2,'owner')", [supplierLeadId, saved.id]);
  return NextResponse.json({ ok: true, message: saved });
}
