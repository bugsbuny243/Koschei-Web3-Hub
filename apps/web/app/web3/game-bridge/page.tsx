import Link from "next/link";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge";

export default function GameBridgePage() {
  return <main className="mx-auto max-w-5xl p-6 space-y-4">
    <h1 className="text-3xl font-bold">Koschei Web3 Game Bridge</h1>
    <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>
    <p>AI-assisted tooling MVP for Godot Web3 development. Build project specs, inventory items, NFT metadata JSON, adapter configs, and integration snippets.</p>
    <div className="flex flex-wrap gap-3">{([
      ["Projects", "/web3/game-bridge/projects"],
      ["New Project", "/web3/game-bridge/projects/new"],
      ["Items", "/web3/game-bridge/items"],
      ["New Item", "/web3/game-bridge/items/new"],
      ["Plugin Export", "/web3/game-bridge/plugin"],
      ["Grant", "/web3/game-bridge/grant"]
    ] as Array<[string, string]>).map(([label, href]) => <Link key={href} href={href} className="underline">{label}</Link>)}</div>
  </main>;
}
