import { NextRequest, NextResponse } from "next/server";
import { web3Env } from "@/lib/web3-env";
import { web3Db } from "@/lib/web3-db";

export async function POST(req: NextRequest) {
  const payload = await req.json().catch(() => null);
  if (!payload) return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });

  const signatureHeader = req.headers.get("x-alchemy-signature");
  const verification = web3Env.ALCHEMY_WEBHOOK_SIGNING_KEY
    ? { enabled: true, receivedSignature: Boolean(signatureHeader) }
    : { enabled: false };

  await web3Db.query(
    `insert into web3_accounting_entries (entry_type, payload) values ('alchemy_webhook_received', $1)`,
    [JSON.stringify({ verification, payload })]
  );

  return NextResponse.json({ ok: true, verification });
}
