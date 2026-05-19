import Link from "next/link";
import { gameBridgePositioning, gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";
import { SupportCta } from "@/components/support-cta";

export default function GameBridgeGrantPage() {
  return <main className="mx-auto max-w-4xl space-y-6 p-6">
    <h1 className="text-3xl font-bold">Koschei Web3 Game Bridge Grant Overview</h1>
    <p className="text-gray-700">{gameBridgePositioning}</p>

    <section><h2 className="text-xl font-semibold">Project description</h2><p className="text-gray-700">Koschei Web3 Game Bridge is a public MVP focused on no-custody developer tooling that helps game studios structure game items and prepare interoperable metadata + adapter outputs.</p></section>
    <section><h2 className="text-xl font-semibold">Problem</h2><p className="text-gray-700">Game teams often struggle with fragmented pipelines for item ownership modeling, metadata normalization, and engine integration.</p></section>
    <section><h2 className="text-xl font-semibold">Solution</h2><p className="text-gray-700">A Godot-first bridge that standardizes item schemas, metadata generation, adapter configs, and script templates through an AI-assisted workflow.</p></section>
    <section><h2 className="text-xl font-semibold">Why Arbitrum</h2><p className="text-gray-700">Arbitrum offers an ecosystem aligned with game-scale throughput and lower operational friction, making it suitable for metadata and ownership adapter planning.</p></section>

    <section>
      <h2 className="text-xl font-semibold">MVP scope</h2>
      <ul className="ml-6 list-disc space-y-1 text-gray-700">
        <li>Item definition + metadata JSON generation demo.</li>
        <li>Arbitrum-targeted read-only adapter configuration output.</li>
        <li>Godot plugin architecture and integration script plan.</li>
      </ul>
    </section>

    <section>
      <h2 className="text-xl font-semibold">Milestones</h2>
      <ul className="ml-6 list-disc space-y-1 text-gray-700">
        <li>M1: Item schema and NFT metadata builder UX.</li>
        <li>M2: Adapter config generator and export templates.</li>
        <li>M3: Godot plugin packaging draft and reviewer docs.</li>
      </ul>
    </section>

    <section><h2 className="text-xl font-semibold">Team</h2><p className="text-gray-700">Koschei product + engineering team building PayWatch and Game Bridge tracks, with Game Bridge prioritized as the main public grant direction.</p></section>

    <section className="rounded bg-amber-100 p-3 text-sm"><h2 className="font-semibold">Safety / no-custody statement</h2><p>{gameBridgeSafetyCopy}</p></section>

    <SupportCta />

    <div className="flex gap-4 text-sm underline">
      <Link href="/web3/game-bridge">Back to Game Bridge</Link>
      <Link href="/web3/game-bridge/plugin">View Plugin Plan</Link>
      <Link href="/web3/game-bridge/items/new">Create Demo Item</Link>
    </div>
  </main>;
}
