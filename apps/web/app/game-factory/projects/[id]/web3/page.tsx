import { notFound } from "next/navigation";
import { buildNftMetadata, buildWeb3BridgeConfig, gameFactoryDb } from "@/lib/game-factory";

export default async function GameFactoryWeb3Page({ params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return notFound();
  const metadata = buildNftMetadata(project.extracted_items ?? []);
  const config = buildWeb3BridgeConfig(project.id);
  return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Web3-ready Package</h1><pre className="overflow-auto rounded bg-gray-100 p-3 text-xs">{JSON.stringify({ metadata, config }, null, 2)}</pre></main>;
}
