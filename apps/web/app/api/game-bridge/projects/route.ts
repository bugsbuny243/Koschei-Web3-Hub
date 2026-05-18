import { NextResponse } from "next/server";
import { gameBridgeDb } from "@/lib/game-bridge";

export async function GET() {
  const rows = await gameBridgeDb.listProjects();
  return NextResponse.json({ projects: rows });
}

export async function POST(req: Request) {
  const body = await req.json();
  if (!body?.name || !body?.slug) return NextResponse.json({ error: "name and slug are required" }, { status: 400 });
  const created = await gameBridgeDb.createProject(body);
  return NextResponse.json({ project: created }, { status: 201 });
}
