import { NextRequest, NextResponse } from "next/server";
import { web3Env } from "@/lib/web3-env";

export async function POST(req: NextRequest) {
  const payload = await req.json().catch(() => null);
  if (!payload) return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });

  const signatureHeader = req.headers.get("x-alchemy-signature");
  const verification = web3Env.ALCHEMY_WEBHOOK_SIGNING_KEY
    ? { enabled: true, receivedSignature: Boolean(signatureHeader), verified: false }
    : { enabled: false, verified: false };

  return NextResponse.json({ ok: true, verification, note: "Verification placeholder only; no accounting write performed." });
}
