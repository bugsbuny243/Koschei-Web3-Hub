import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { analyzeQuoteRequestWithAi } from "@/lib/ai/tradepi-ai";
import { appendMilestone, isOwnerRequest } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const form = await req.formData().catch(() => null);
  const body = form ? Object.fromEntries(form.entries()) : await req.json().catch(() => ({}));
  const password = (body.password as string) ?? null;
  if (!isOwnerRequest(password)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });

  const quoteRequestId = String(body.quote_request_id || "");
  const pool = getDbPool();
  if (!pool || !quoteRequestId) return NextResponse.json({ error: "missing quote_request_id" }, { status: 400 });

  const rfq = (await pool.query("select * from quote_requests where id=$1", [quoteRequestId])).rows[0];
  if (!rfq) return NextResponse.json({ error: "rfq not found" }, { status: 404 });

  const analysis = await analyzeQuoteRequestWithAi({ quoteRequest: rfq });
  await pool.query("insert into ai_rfq_analyses (quote_request_id,analysis_json,created_by) values ($1,$2,$3)", [quoteRequestId, JSON.stringify(analysis), "owner"]);
  await appendMilestone(quoteRequestId, "AI analyzed");
  return NextResponse.json({ ok: true, quote_request_id: quoteRequestId, analysis });
}
