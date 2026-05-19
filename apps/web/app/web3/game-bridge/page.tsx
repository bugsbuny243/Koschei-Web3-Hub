import Link from "next/link";
import { gameBridgePositioning, gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";
import { SupportCta } from "@/components/support-cta";

export default function GameBridgePage() {
  return <main className="mx-auto max-w-6xl space-y-6 px-4 py-6 sm:px-6 sm:space-y-8">
    <section className="space-y-4 rounded-xl border bg-gradient-to-r from-violet-50 to-indigo-50 p-5 sm:p-6">
      <p className="text-xs font-semibold uppercase tracking-wide text-violet-700">Primary Grant Direction</p>
      <h1 className="text-3xl font-bold sm:text-4xl">Koschei Web3 Bridge</h1>
      <p className="max-w-3xl text-gray-700">{gameBridgePositioning}</p>
      <div className="flex flex-wrap gap-3 pt-2">
        <Link href="/web3/game-bridge/items/new" className="rounded bg-black px-4 py-3 text-sm font-semibold text-white">Create Demo Item</Link>
        <Link href="/web3/game-bridge/grant" className="rounded border px-4 py-3 text-sm font-semibold">View Grant Overview</Link>
        <Link href="/web3/game-bridge/plugin" className="rounded border px-4 py-3 text-sm font-semibold">View Plugin Plan</Link>
      </div>
    </section>

    <section className="grid gap-4 sm:grid-cols-2">
      <article className="rounded-xl border p-4 sm:p-5"><h3 className="text-base font-semibold">Godot Web3 Bridge</h3><p className="mt-1 text-sm text-gray-700">Plugin panel and scripts for integrating item schema and metadata workflows into Godot projects.</p></article>
      <article className="rounded-xl border p-4 sm:p-5"><h3 className="text-base font-semibold">NFT metadata builder</h3><p className="mt-1 text-sm text-gray-700">Generates NFT metadata JSON from gameplay item definitions and attributes.</p></article>
      <article className="rounded-xl border p-4 sm:p-5"><h3 className="text-base font-semibold">Arbitrum Sepolia adapter prep</h3><p className="mt-1 text-sm text-gray-700">Produces read-only adapter configuration for integration testing on Arbitrum Sepolia.</p></article>
      <article className="rounded-xl border p-4 sm:p-5"><h3 className="text-base font-semibold">AI-assisted integration</h3><p className="mt-1 text-sm text-gray-700">Provides AI-assisted script scaffolding for developer-side integration workflows.</p></article>
    </section>

    <section className="rounded bg-amber-100 p-4 text-sm">
      <h2 className="mb-1 text-base font-semibold">MVP safety boundary</h2>
      <p>{gameBridgeSafetyCopy}</p>
    </section>

    <SupportCta />
  </main>;
}
