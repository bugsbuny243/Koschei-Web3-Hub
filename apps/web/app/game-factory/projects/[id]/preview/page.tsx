export const dynamic = 'force-dynamic';
import Link from "next/link";
import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";
import { GameFactoryGenerateButton } from "@/components/game-factory-generate-button";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const p = await gameFactoryDb.getProject(id);
  if (!p) return notFound();
  const files = await gameFactoryDb.getFiles(p.id);
  const html = (files.find((f) => f.file_type === "html") as { content?: string } | undefined)?.content;

  return (
    <main className="mx-auto max-w-5xl space-y-4 p-6">
      <h1 className="text-3xl font-bold">Live Preview</h1>

      <div className="flex flex-wrap gap-2">
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${p.id}`}>
          Project Detail
        </Link>
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${p.id}/web3`}>
          Web3 Package
        </Link>
        <Link className="rounded border px-3 py-2" href="/game-factory/projects">
          Back to Projects
        </Link>
        <Link className="rounded border px-3 py-2" href="/game-factory/new">
          Create Another Game
        </Link>
      </div>

      {html ? (
        <iframe title="preview" className="h-[420px] w-full rounded border" sandbox="allow-scripts" srcDoc={html} />
      ) : (
        <div className="space-y-3 rounded border p-4">
          <p>No generated game preview yet.</p>
          <GameFactoryGenerateButton id={p.id} kind="preview" />
        </div>
      )}
    </main>
  );
}
