import Link from "next/link";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";
import { SupportCta } from "@/components/support-cta";

export default function GameBridgeGrantPage() {
  return <main className="mx-auto max-w-4xl space-y-6 px-4 py-6 sm:px-6">
    <h1 className="text-3xl font-bold">Koschei Web Game Factory + Web3 Bridge Grant Overview</h1>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Project description</h2>
      <p className="mt-2 text-gray-700">Koschei Web Game Factory + Web3 Bridge is a no-custody developer tooling MVP that helps teams move from game prompt to playable demo and then prepare interoperable Web3-ready artifacts.</p>
    </section>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Scope focus</h2>
      <ul className="mt-2 ml-6 list-disc space-y-1 text-gray-700">
        <li>Prompt-to-playable game generation and live preview workflow.</li>
        <li>Item schemas, reward configs, and NFT metadata preparation.</li>
        <li>Godot Web3 Bridge plugin plan and Arbitrum Sepolia adapter configuration outputs.</li>
      </ul>
    </section>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Team</h2>
      <p className="mt-2 text-gray-700">Koschei is currently built by a solo founder using AI-assisted development workflows and cloud-native infrastructure. The MVP is intentionally scoped as no-custody developer tooling and does not handle user funds, private keys, wallet connections, minting, or contract deployment.</p>
    </section>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Supporting context</h2>
      <p className="mt-2 text-gray-700">PayWatch may continue as an experimental supporting module, but it is not the main grant direction.</p>
    </section>

    <section className="rounded bg-amber-100 p-3 text-sm"><h2 className="font-semibold">Safety / no-custody statement</h2><p>{gameBridgeSafetyCopy}</p></section>

    <SupportCta />

    <div className="flex flex-wrap gap-3 text-sm">
      <Link className="underline" href="/web3/game-bridge">Back to Web3 Bridge</Link>
      <Link className="underline" href="/web3/game-bridge/plugin">View Plugin Plan</Link>
      <Link className="underline" href="/web3/game-bridge/items/new">Create Demo Item</Link>
    </div>
  </main>;
}
