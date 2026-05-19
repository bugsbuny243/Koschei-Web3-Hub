import Link from "next/link";
import { gameFactoryDb } from "@/lib/game-factory";

export default async function GameFactoryProjectsPage() {
  const projects = await gameFactoryDb.listProjects();
  return <main className="mx-auto max-w-5xl p-6"><h1 className="mb-4 text-3xl font-bold">Game Factory Projects</h1><div className="space-y-3">{projects.map((p) => <Link key={p.id} href={`/game-factory/projects/${p.id}`} className="block rounded border p-3 hover:bg-gray-50"><div className="font-semibold">{p.name}</div><div className="text-sm text-gray-600">{p.status} · {p.slug}</div></Link>)}</div></main>;
}
