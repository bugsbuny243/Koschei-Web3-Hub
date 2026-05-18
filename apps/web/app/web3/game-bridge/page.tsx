import Link from "next/link";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge";

const demoRoutes: Array<{ label: string; href: string; description: string }> = [
  {
    label: "Grant Demo Overview",
    href: "/web3/game-bridge/grant",
    description: "Public grant narrative, scope, and milestone plan for reviewers."
  },
  {
    label: "Plugin + Adapter Preview",
    href: "/web3/game-bridge/plugin",
    description: "Generated plugin payload with metadata and adapter configuration outputs."
  },
  {
    label: "Create Item Metadata",
    href: "/web3/game-bridge/items/new",
    description: "Interactive item JSON editor to generate game item ownership metadata inputs."
  }
];

export default function GameBridgePage() {
  return <main className="mx-auto max-w-5xl space-y-6 p-6">
    <header className="space-y-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-violet-700">Primary Grant Direction</p>
      <h1 className="text-3xl font-bold">Koschei Web3 Game Bridge</h1>
      <p className="text-base text-gray-700">AI-assisted no-custody Godot Web3 developer tooling for game item ownership, NFT metadata, adapter configs, and integration scripts.</p>
      <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>
    </header>

    <section className="grid gap-4 md:grid-cols-3">
      {demoRoutes.map((route) => (
        <Link key={route.href} href={route.href} className="rounded-lg border p-4 transition hover:bg-gray-50">
          <h2 className="text-lg font-semibold">{route.label}</h2>
          <p className="mt-2 text-sm text-gray-600">{route.description}</p>
          <p className="mt-3 text-sm underline">Open demo</p>
        </Link>
      ))}
    </section>
  </main>;
}
