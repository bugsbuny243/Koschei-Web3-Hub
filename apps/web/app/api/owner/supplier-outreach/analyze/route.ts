import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";
import { analyzeSupplierLead } from "@/lib/ai/supplier-analysis-ai";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const supplierLeadId = String(body.supplier_lead_id || "");
  if (!process.env.TOGETHER_API_KEY) return NextResponse.json({ error: "Together API is not configured." }, { status: 400 });
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "Database is not configured." }, { status: 500 });
  if (!supplierLeadId) return NextResponse.json({ error: "missing supplier_lead_id" }, { status: 400 });

  const lead = (await pool.query("select * from supplier_leads where id=$1", [supplierLeadId])).rows[0];
  const source = (await pool.query("select * from supplier_lead_sources where supplier_lead_id=$1 order by created_at asc limit 1", [supplierLeadId])).rows[0];
  if (!lead || !source) return NextResponse.json({ error: "lead/source not found" }, { status: 404 });

  try {
    const analysis = await analyzeSupplierLead({ title: source.source_title, url: source.source_url, snippet: source.source_snippet, platform: source.platform });
    await pool.query(
      "insert into supplier_ai_analyses (supplier_lead_id,raw_ai_output,likely_manufacturer,likely_trader,verified_claim_found,product_fit,manufacturer_score,risk_score,contact_possible,contact_method,risk_notes,recommended_action,confidence) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)",
      [supplierLeadId, JSON.stringify(analysis), analysis.likely_manufacturer, analysis.likely_trader, analysis.verified_claim_found, analysis.product_fit, analysis.manufacturer_score, analysis.risk_score, analysis.contact_possible, analysis.contact_method, analysis.risk_notes, analysis.recommended_action, analysis.confidence],
    );
    await pool.query("update supplier_leads set company_name=$2,possible_company_name=$3,country=$4,city=$5,is_verified_claimed=$6,likely_manufacturer=$7,likely_trader=$8,manufacturer_score=$9,risk_score=$10,confidence=$11,updated_at=now() where id=$1", [supplierLeadId, analysis.company_name || lead.company_name || null, analysis.possible_company_name || lead.possible_company_name || null, analysis.country || lead.country, analysis.city || null, analysis.verified_claim_found, analysis.likely_manufacturer, analysis.likely_trader, analysis.manufacturer_score, analysis.risk_score, analysis.confidence]);
    await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,'analyzed',$2,'owner')", [supplierLeadId, analysis.confidence]);

    return NextResponse.json({ ok: true, analysis });
  } catch {
    return NextResponse.json({ error: "AI analysis failed" }, { status: 500 });
  }
}
