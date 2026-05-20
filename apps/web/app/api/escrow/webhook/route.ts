import { NextRequest, NextResponse } from "next/server";
import { fetchEscrowTransaction } from "@/lib/escrow-client";
import { escrowTransactions, webhookEvents } from "@/lib/payment-store";

function getToken(req: NextRequest) {
  return req.headers.get("x-escrow-webhook-token") ?? req.nextUrl.searchParams.get("token");
}

export async function POST(request: NextRequest) {
  const expected = process.env.ESCROW_WEBHOOK_TOKEN;
  if (!expected || getToken(request) !== expected) {
    return NextResponse.json({ error: "Invalid webhook token" }, { status: 401 });
  }

  const payload = await request.json();
  webhookEvents.push({ raw_payload: payload, received_at: new Date().toISOString(), verified_by_fetch: false });

  const transactionId = payload?.transaction_id ?? payload?.id;
  if (!transactionId) return NextResponse.json({ ok: true, warning: "transaction id missing" });

  const fetched = await fetchEscrowTransaction(transactionId);
  const record = escrowTransactions.find((x) => x.escrow_transaction_id === transactionId);
  if (record) {
    record.escrow_status = fetched.status ?? record.escrow_status;
    record.raw_response = fetched;
    record.updated_at = new Date().toISOString();
  }
  webhookEvents[webhookEvents.length - 1].verified_by_fetch = true;

  return NextResponse.json({ ok: true });
}
