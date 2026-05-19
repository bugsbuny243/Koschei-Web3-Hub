import { NextResponse } from "next/server";
import { buildMetadata } from "@/lib/game-bridge";

export const runtime = "nodejs";
export async function POST(req: Request) {
  const body = await req.json();
  const metadata = buildMetadata(body);
  return NextResponse.json({ metadata });
}
