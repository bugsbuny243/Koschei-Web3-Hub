import { NextResponse } from "next/server";

export async function POST(req: Request) {
  const body = (await req.json()) as { prompt?: string };
  if (!body.prompt) {
    return NextResponse.json({ error: "prompt is required" }, { status: 400 });
  }

  return NextResponse.json({
    ok: true,
    message: "Agent created",
    agent: {
      name: "AutoYieldOptimizerAgent",
      prompt: body.prompt,
      status: "queued"
    }
  });
}
