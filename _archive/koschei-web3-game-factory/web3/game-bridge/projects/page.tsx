import { gameBridgeDb } from "@/lib/game-bridge";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export default async function ProjectsPage() {
  try {
    const projects = await gameBridgeDb.listProjects();
    return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Game Bridge Projects</h1>
      <pre className="rounded bg-gray-100 p-4 text-xs overflow-auto">{JSON.stringify(projects, null, 2)}</pre>
    </main>;
  } catch {
    return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Game Bridge Projects</h1>
      <p className="rounded border p-4 text-gray-600">Database is not available right now.</p>
    </main>;
  }
}
