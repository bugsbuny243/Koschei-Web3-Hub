import Link from "next/link";
import { gameBridgePositioning, gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";
import { SupportCta } from "@/components/support-cta";

export default function GameBridgePage() {
  return <main className="mx-auto max-w-6xl space-y-8 p-6">
    <section className="space-y-4 rounded-xl border bg-gradient-to-r from-violet-50 to-indigo-50 p-6">
      <p className="text-xs font-semibold uppercase tracking-wide text-violet-700">Primary Grant Direction</p>
      <h1 className="text-4xl font-bold">Koschei Web3 Game Bridge</h1>
      <p className="max-w-3xl text-gray-700">{gameBridgePositioning}</p>
      <div className="flex flex-wrap gap-3 pt-2">
        <Link href="/web3/game-bridge/items/new" className="rounded bg-black px-4 py-2 text-white">Create Demo Item</Link>
        <Link href="/web3/game-bridge/grant" className="rounded border px-4 py-2">View Grant Overview</Link>
        <Link href="/web3/game-bridge/plugin" className="rounded border px-4 py-2">View Plugin Plan</Link>
      </div>
    </section>

    <section className="space-y-2">
      <h2 className="text-2xl font-semibold">What it does</h2>
      <p className="text-gray-700">Generates structured game item ownership data, NFT-compatible metadata JSON, Arbitrum adapter configs, and AI-assisted Godot integration scripts so game teams can ship faster without wallet custody risk.</p>
    </section>

    <section className="space-y-2">
      <h2 className="text-2xl font-semibold">How it works</h2>
      <ol className="ml-6 list-decimal space-y-1 text-gray-700">
        <li>Design game items with rarity, attributes, and media references.</li>
        <li>Generate deterministic metadata JSON preview for review and iteration.</li>
        <li>Prepare Arbitrum adapter settings and Godot plugin-side integration stubs.</li>
        <li>Export JSON and scripts for developer-controlled deployment workflows.</li>
      </ol>
    </section>

    <section className="rounded bg-amber-100 p-4 text-sm">
      <h2 className="mb-1 text-lg font-semibold">No-custody safety notice</h2>
      <p>{gameBridgeSafetyCopy}</p>
    </section>

    <section className="grid gap-4 md:grid-cols-2">
      <article className="rounded border p-4"><h3 className="font-semibold">Godot plugin</h3><p className="text-sm text-gray-700">Plugin panel and core scripts for integrating item metadata pipelines into Godot game projects.</p></article>
      <article className="rounded border p-4"><h3 className="font-semibold">NFT metadata generator</h3><p className="text-sm text-gray-700">Builds NFT-compatible JSON with trait arrays from gameplay item definitions and attributes.</p></article>
      <article className="rounded border p-4"><h3 className="font-semibold">Arbitrum adapter preparation</h3><p className="text-sm text-gray-700">Produces read-only adapter configuration values for Arbitrum-aligned indexing and metadata flows.</p></article>
      <article className="rounded border p-4"><h3 className="font-semibold">AI integration assistant</h3><p className="text-sm text-gray-700">Guides developers with integration script scaffolding and adapter wiring recommendations.</p></article>
    </section>

    <section className="rounded border p-4 text-sm">
      <p>{gameBridgeSafetyCopy}</p>
    </section>

    <SupportCta />
  </main>;
}
