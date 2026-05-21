import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const messageId = String(body.message_id || "");
  const pool = getDbPool();
  if (!pool || !messageId) return NextResponse.json({ error: "missing message_id" }, { status: 400 });
  await pool.query("update supplier_outreach_messages set approved_by_owner=true,updated_at=now() where id=$1", [messageId]);
  return NextResponse.json({ ok: true });
}
