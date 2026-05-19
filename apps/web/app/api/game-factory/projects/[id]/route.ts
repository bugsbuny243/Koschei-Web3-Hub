import { NextResponse } from "next/server";
import { gameFactoryDb } from "@/lib/game-factory";

export async function GET(_req: Request, { params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return NextResponse.json({ error: "not found" }, { status: 404 });
  return NextResponse.json({ project });
}
