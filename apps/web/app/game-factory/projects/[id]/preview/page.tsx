import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";

export default async function GameFactoryPreviewPage({ params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return notFound();
  return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Live Preview</h1><p className="text-sm text-gray-600">Prototype JS output for Phaser:</p><pre className="overflow-auto rounded bg-black p-3 text-xs text-green-300">{project.phaser_template ?? "Run generation first via API POST /api/game-factory/projects/[id]/generate"}</pre></main>;
}
