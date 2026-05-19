import { NextResponse } from "next/server";
import { buildGameBrief } from "@/lib/game-factory";

export async function POST(req: Request) {
  const body = await req.json();
  if (!body?.prompt) return NextResponse.json({ error: "prompt is required" }, { status: 400 });
  return NextResponse.json({ brief: buildGameBrief(body.prompt) });
}
