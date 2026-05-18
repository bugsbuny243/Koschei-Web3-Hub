import { NextRequest, NextResponse } from "next/server";
import { web3Env } from "@/lib/web3-env";
import { processPaymentEvent } from "@/lib/web3-service";

export async function POST(req: NextRequest) {
  if (req.headers.get("x-webhook-secret") !== web3Env.WEBHOOK_SECRET) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await req.json();
  const result = await processPaymentEvent({
    chain_slug: body.chain_slug,
    tx_hash: body.tx_hash,
    log_index: body.log_index,
    from_address: body.from_address,
    to_address: body.to_address,
    token_contract: body.token_contract,
    token_symbol: body.token_symbol,
    token_decimals: Number(body.token_decimals),
    amount: String(body.amount),
    raw_amount: body.raw_amount,
    block_number: body.block_number,
    raw_event: body.raw_event
  });

  return NextResponse.json({ ok: true, ...result });
}
