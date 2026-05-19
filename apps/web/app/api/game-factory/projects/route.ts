import { NextResponse } from "next/server";
import { gameFactoryDb } from "@/lib/game-factory";

export async function GET() {
  const projects = await gameFactoryDb.listProjects();
  return NextResponse.json({ projects });
}

export async function POST(req: Request) {
  const body = await req.json();
  if (!body?.name || !body?.slug || !body?.prompt) return NextResponse.json({ error: "name, slug, prompt are required" }, { status: 400 });
  const project = await gameFactoryDb.createProject(body);
  return NextResponse.json({ project }, { status: 201 });
}
