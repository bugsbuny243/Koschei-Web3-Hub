import Link from "next/link";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge";

export default function GameBridgeGrantPage() {
  return <main className="mx-auto max-w-4xl space-y-5 p-6">
    <h1 className="text-3xl font-bold">Koschei Web3 Game Bridge - Public Grant Demo</h1>
    <p className="text-gray-700">Koschei Web3 Game Bridge is the primary grant direction for the Koschei Web3 suite.</p>
    <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>

    <section className="space-y-2">
      <h2 className="text-xl font-semibold">Product Positioning</h2>
      <p>AI-assisted no-custody Godot Web3 developer tooling for game item ownership, NFT metadata, adapter configs, and integration scripts.</p>
    </section>

    <section className="space-y-2">
      <h2 className="text-xl font-semibold">Grant Milestones</h2>
      <ul className="ml-6 list-disc space-y-1">
        <li>Milestone 1: Structured game item ownership schemas and metadata templates.</li>
        <li>Milestone 2: Adapter config generation and Godot integration script assistant.</li>
        <li>Milestone 3: Public plugin packaging flow and reviewer-ready validation checklist.</li>
      </ul>
    </section>

    <section className="space-y-2">
      <h2 className="text-xl font-semibold">Public Demo Routes</h2>
      <div className="flex flex-wrap gap-4">
        <Link className="underline" href="/web3/game-bridge">Overview</Link>
        <Link className="underline" href="/web3/game-bridge/plugin">Plugin Demo</Link>
        <Link className="underline" href="/web3/game-bridge/items/new">New Item Demo</Link>
      </div>
    </section>
  </main>;
}
