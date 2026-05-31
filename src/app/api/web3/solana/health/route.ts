import { NextResponse } from "next/server";
import { checkChainHealth } from "@/lib/web3/alchemy";

/** @deprecated Use /api/web3/health?chain=solana. */
export async function GET(){return NextResponse.json(await checkChainHealth("solana"));}
