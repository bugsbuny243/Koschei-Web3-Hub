import { NextResponse } from "next/server";
import { buildGameBrief, buildPhaserTemplate, extractItemsFromBrief, gameFactoryDb } from "@/lib/game-factory";

export async function POST(_req: Request, { params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return NextResponse.json({ error: "not found" }, { status: 404 });
  const brief = buildGameBrief(project.prompt);
  const phaserTemplate = buildPhaserTemplate(project.id, brief);
  const extractedItems = extractItemsFromBrief(brief);
  const updated = await gameFactoryDb.saveGenerated(project.id, brief, phaserTemplate, extractedItems);
  return NextResponse.json({ project: updated });
}
