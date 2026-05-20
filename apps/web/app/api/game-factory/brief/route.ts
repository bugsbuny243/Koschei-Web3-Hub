import { NextResponse } from "next/server";
import { buildGameBrief } from "@/lib/game-factory";

export const runtime = "nodejs";
export async function POST(req: Request) {
  const body = await req.json();
  if (!body?.prompt) return NextResponse.json({ error: "prompt is required" }, { status: 400 });
  const brief = buildGameBrief({
    title: body.title,
    prompt: body.prompt,
    genre: body.genre,
    style: body.style
  });
  return NextResponse.json({ ok: true, brief });
}
