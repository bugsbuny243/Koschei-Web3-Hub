export const runtime = "nodejs";
export const dynamic = 'force-dynamic';
import Link from "next/link";
import { gameFactoryDb } from "@/lib/game-factory";

export default async function Page(){try{const projects=await gameFactoryDb.listProjects();return <main className="mx-auto max-w-5xl p-6"><h1 className="mb-4 text-3xl font-bold">Game Factory Projects</h1>{projects.length===0?<p className="rounded border p-4 text-gray-600">No projects yet. Create your first prompt-to-game project.</p>:<div className="space-y-3">{projects.map(p=><Link key={p.id} href={`/game-factory/projects/${p.id}`} className="block rounded border p-3"><div className="font-semibold">{p.title||"Untitled project"}</div><div className="text-sm text-gray-600">{p.status} · {p.target_chain}</div></Link>)}</div>}</main>;}catch{return <main className="mx-auto max-w-5xl p-6"><h1 className="mb-4 text-3xl font-bold">Game Factory Projects</h1><p className="rounded border p-4 text-gray-600">Database is not available right now.</p></main>;}}
