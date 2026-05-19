import Link from "next/link";
import { gameFactoryPositioning, gameFactorySafetyCopy } from "@/lib/game-factory";

export default function GameFactoryPage() {
  return <main className="mx-auto max-w-6xl space-y-8 p-6">
    <section className="space-y-4 rounded-xl border bg-gradient-to-r from-emerald-50 to-cyan-50 p-6">
      <p className="text-xs font-semibold uppercase tracking-wide text-emerald-700">Main Koschei Direction</p>
      <h1 className="text-4xl font-bold">Koschei Web Game Factory + Web3 Bridge</h1>
      <p className="max-w-3xl text-gray-700">{gameFactoryPositioning}</p>
      <div className="flex flex-wrap gap-3 pt-2">
        <Link href="/game-factory/new" className="rounded bg-black px-4 py-2 text-white">New Project</Link>
        <Link href="/game-factory/projects" className="rounded border px-4 py-2">Projects</Link>
        <Link href="/game-factory/grant" className="rounded border px-4 py-2">Grant</Link>
      </div>
    </section>
    <section className="rounded bg-amber-100 p-4 text-sm">{gameFactorySafetyCopy}</section>
  </main>;
}
