import { NextRequest, NextResponse } from "next/server";
import { fetchEscrowTransaction } from "@/lib/escrow-client";
import { getDbPool } from "@/lib/db";

function getToken(req: NextRequest) { return req.headers.get("x-escrow-webhook-token") ?? req.nextUrl.searchParams.get("token"); }

export async function POST(request: NextRequest) {
  if (!process.env.ESCROW_WEBHOOK_TOKEN || getToken(request) !== process.env.ESCROW_WEBHOOK_TOKEN) return NextResponse.json({ error: "Invalid webhook token" }, { status: 401 });
  const payload = await request.json(); const pool = getDbPool(); if (!pool) return NextResponse.json({ error: "DB unavailable" }, { status: 500 });
  const transactionId = String(payload?.transaction_id ?? payload?.id ?? "");
  await pool.query("insert into escrow_webhook_events (escrow_transaction_id,event,event_type,raw_payload,verified_by_fetch) values ($1,$2,$3,$4,false)", [transactionId || null, payload?.event ?? null, payload?.event_type ?? null, JSON.stringify(payload)]);
  if (!transactionId) return NextResponse.json({ ok: true, warning: "transaction id missing" });
  const fetched = await fetchEscrowTransaction(transactionId);
  await pool.query("update escrow_transactions set escrow_status=$2, raw_response=$3, updated_at=now() where escrow_transaction_id=$1", [transactionId, fetched.status ?? null, JSON.stringify(fetched)]);
  await pool.query("update escrow_webhook_events set verified_by_fetch=true where escrow_transaction_id=$1", [transactionId]);
  return NextResponse.json({ ok: true });
}
