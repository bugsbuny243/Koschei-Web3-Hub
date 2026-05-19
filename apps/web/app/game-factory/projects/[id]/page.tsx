import Link from "next/link";
import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";

export default async function GameFactoryProjectDetailPage({ params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return notFound();
  return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">{project.name}</h1><p className="text-gray-700">{project.prompt}</p><div className="flex gap-3"><Link href={`/game-factory/projects/${project.id}/preview`} className="rounded border px-3 py-2">Preview</Link><Link href={`/game-factory/projects/${project.id}/web3`} className="rounded border px-3 py-2">Web3 Package</Link></div><pre className="overflow-auto rounded bg-gray-100 p-3 text-xs">{JSON.stringify(project.game_brief, null, 2)}</pre></main>;
}
