import { NextResponse } from "next/server";
import { buildSupplierMessageWithAi } from "@/lib/ai/tradepi-ai";
import { getDbPool } from "@/lib/db";
import { appendMilestone, isOwnerRequest } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const quoteRequestId = String(body.quote_request_id || "");
  const pool = getDbPool();
  if (!pool || !quoteRequestId) return NextResponse.json({ error: "missing quote_request_id" }, { status: 400 });
  const rfq = (await pool.query("select * from quote_requests where id=$1", [quoteRequestId])).rows[0];
  if (!rfq) return NextResponse.json({ error: "rfq not found" }, { status: 404 });

  const msg = await buildSupplierMessageWithAi({ quoteRequest: rfq });
  await pool.query("insert into supplier_messages (quote_request_id,message_json) values ($1,$2)", [quoteRequestId, JSON.stringify(msg)]);
  await appendMilestone(quoteRequestId, "supplier message sent");
  return NextResponse.json({ ok: true, message: msg });
}
