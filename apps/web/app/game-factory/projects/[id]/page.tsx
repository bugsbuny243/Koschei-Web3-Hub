export const runtime = "nodejs";
export const dynamic = "force-dynamic";

import Link from "next/link";
import { notFound } from "next/navigation";
import { web3Db } from "@/lib/web3-db";

type PageProps = {
  params: Promise<{ id: string }>;
};

type ProjectRow = {
  id: string;
  title: string | null;
  prompt: string;
  genre: string | null;
  visual_style: string | null;
  target_chain: string;
  status: string;
};

export default async function ProjectDetailPage({ params }: PageProps) {
  const { id } = await params;

  const result = await web3Db.query<ProjectRow>(
    `select * from game_factory_projects where id = $1 limit 1`,
    [id]
  );

  const project = result.rows[0] ?? null;

  if (!project) {
    notFound();
  }

  return (
    <main className="mx-auto max-w-5xl space-y-4 p-6">
      <h1 className="text-3xl font-bold">{project.title || "Untitled project"}</h1>
      <p>{project.prompt}</p>

      <div className="grid gap-2 rounded border p-4 text-sm">
        <p>
          <span className="font-semibold">Genre:</span> {project.genre || "-"}
        </p>
        <p>
          <span className="font-semibold">Visual style:</span> {project.visual_style || "-"}
        </p>
        <p>
          <span className="font-semibold">Target chain:</span> {project.target_chain}
        </p>
        <p>
          <span className="font-semibold">Status:</span> {project.status}
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${project.id}/preview`}>
          Live Preview
        </Link>
        <Link className="rounded border px-3 py-2" href={`/game-factory/projects/${project.id}/web3`}>
          Web3 Package
        </Link>
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
