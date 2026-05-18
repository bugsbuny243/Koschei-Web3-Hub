import { NextResponse } from "next/server";
import { web3Db } from "@/lib/web3-db";

export async function GET() {
  const chains = await web3Db.chains.listActive();
  return NextResponse.json({ chains });
}
