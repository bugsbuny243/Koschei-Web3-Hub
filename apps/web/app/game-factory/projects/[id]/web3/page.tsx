export const dynamic = "force-dynamic";

import Link from "next/link";
import { gameFactoryDb, gameFactorySafetyCopy } from "@/lib/game-factory";
import { GameFactoryGenerateButton } from "@/components/game-factory-generate-button";

function JsonBlock({ title, data }: { title: string; data: unknown }) {
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">{title}</h2>
      <textarea
        readOnly
        className="min-h-40 w-full rounded border bg-gray-50 p-2 font-mono text-xs"
        value={JSON.stringify(data ?? {}, null, 2)}
      />
    </section>
  );
}

type PageProps = {
  params: Promise<{ id: string }>;
};

export default async function Web3PackagePage({ params }: PageProps) {
  const { id } = await params;
  const project = await gameFactoryDb.getProject(id);
  console.error("[web3-package-page]", { id, found: Boolean(project) });

  if (!project) {
    return (
      <main className="mx-auto max-w-5xl space-y-4 p-6">
        <h1 className="text-3xl font-bold">Web3 Package</h1>
        <p>Project not found</p>
        <div className="flex flex-wrap gap-2">
          <Link className="rounded border px-3 py-2" href="/game-factory/projects">
            Back to Projects
          </Link>
          <Link className="rounded border px-3 py-2" href="/game-factory/new">
            Create Another Game
          </Link>
        </div>
      </main>
    );
  }

  const pkg = await gameFactoryDb.getWeb3Package(project.id);

  return (
    <main className="mx-auto max-w-5xl space-y-4 p-6">
      <h1 className="text-3xl font-bold">Web3 Package</h1>
      <h2 className="text-xl font-semibold">{project.title || "Untitled project"}</h2>
      <p>{project.prompt}</p>
      <p className="text-sm text-gray-600">Target chain: {project.target_chain}</p>

      <div className="flex flex-wrap gap-2">
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${project.id}`}>
          Project Detail
        </Link>
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${project.id}/preview`}>
          Live Preview
        </Link>
        <Link className="rounded border px-3 py-2" href="/game-factory/projects">
          Back to Projects
        </Link>
        <Link className="rounded border px-3 py-2" href="/game-factory/new">
          Create Another Game
        </Link>
      </div>

      <p className="rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p>

      {!pkg ? (
        <div className="space-y-3 rounded border p-4">
          <p>No package generated yet.</p>
          <GameFactoryGenerateButton id={project.id} kind="web3" />
        </div>
      ) : (
        <div className="space-y-4">
          <JsonBlock title="Game Manifest" data={pkg.manifest} />
          <JsonBlock title="Item Schema" data={pkg.item_schema} />
          <JsonBlock title="NFT Metadata" data={pkg.nft_metadata} />
          <JsonBlock title="Reward Config" data={pkg.reward_config} />
          <JsonBlock title="Quest / Achievement Config" data={pkg.reward_config} />
          <JsonBlock title="Arbitrum Sepolia Adapter Config" data={pkg.adapter_config} />
        </div>
      )}
    </main>
  );
}
