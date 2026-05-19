import { NextRequest, NextResponse } from "next/server";
import { ethers } from "ethers";
import { web3Env } from "@/lib/web3-env";
import { parseTransferLog, processPaymentEvent } from "@/lib/web3-service";

export const runtime = "nodejs";
const MAX_BLOCK_RANGE = 3000;

export async function POST(req: NextRequest) {
  if (req.headers.get("x-cron-secret") !== web3Env.CRON_SECRET) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await req.json();
  const tokenContract = body.token_contract;
  const tokenSymbol = body.token_symbol ?? "USDC";
  const tokenDecimals = Number(body.token_decimals ?? 6);
  const fromBlock = Number(body.from_block);
  const toBlock = Number(body.to_block);

  if (!tokenContract || Number.isNaN(fromBlock) || Number.isNaN(toBlock)) {
    return NextResponse.json({ error: "Invalid input" }, { status: 400 });
  }
  if (toBlock < fromBlock || toBlock - fromBlock > MAX_BLOCK_RANGE) {
    return NextResponse.json({ error: `Block range too large. Max range is ${MAX_BLOCK_RANGE}` }, { status: 400 });
  }

  const provider = new ethers.JsonRpcProvider(web3Env.ARBITRUM_SEPOLIA_RPC_URL);
  const topic = ethers.id("Transfer(address,address,uint256)");

  const logs = await provider.getLogs({
    address: tokenContract,
    topics: [topic],
    fromBlock,
    toBlock
  });

  let inserted = 0;
  for (const log of logs) {
    const parsed = parseTransferLog(log, tokenDecimals);
    if (!parsed) continue;
    await processPaymentEvent({
      chain_slug: "arbitrum-sepolia",
      tx_hash: log.transactionHash,
      log_index: log.index,
      from_address: parsed.from,
      to_address: parsed.to,
      token_contract: tokenContract,
      token_symbol: tokenSymbol,
      token_decimals: tokenDecimals,
      amount: parsed.amount,
      raw_amount: parsed.rawAmount,
      block_number: log.blockNumber,
      raw_event: { topics: log.topics, data: log.data }
    });
    inserted += 1;
  }

  return NextResponse.json({ ok: true, scanned: logs.length, inserted });
}
