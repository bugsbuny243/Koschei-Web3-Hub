import Link from "next/link";
import { SupportCta } from "@/components/support-cta";
import { gameFactorySafetyCopy } from "@/lib/game-factory";

export default function Home() {
  return <main className="mx-auto max-w-5xl space-y-6 px-4 py-6 sm:px-6">
    <section className="rounded-xl border bg-gradient-to-r from-cyan-50 to-emerald-50 p-5 sm:p-6">
      <h1 className="text-3xl font-bold sm:text-4xl">Koschei Web Game Factory + Web3 Bridge</h1>
      <p className="mt-3 text-base text-gray-700">Prompt → playable HTML5 game → live preview → Web3-ready package.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link className="rounded bg-black px-4 py-3 text-sm font-semibold text-white" href="/game-factory/new">Create Game</Link>
        <Link className="rounded border px-4 py-3 text-sm font-semibold" href="/web3/game-bridge">View Web3 Bridge</Link>
        <a className="rounded border border-emerald-700 px-4 py-3 text-sm font-semibold text-emerald-700" href="https://www.shopier.com/TradeVisual/47208457" target="_blank" rel="noopener noreferrer">Support with 10 TL</a>
      </div>
    </section>

    <section className="rounded-xl border p-5 sm:p-6">
      <h2 className="text-xl font-semibold">What it does</h2>
      <p className="mt-2 text-gray-700">Koschei Web Game Factory turns plain-language prompts into a playable browser demo and prepares a Web3-ready package for downstream developer workflows.</p>
    </section>

    <section className="rounded-xl border p-5 sm:p-6">
      <h2 className="text-xl font-semibold">How it works</h2>
      <ol className="mt-2 ml-5 list-decimal space-y-2 text-gray-700">
        <li>Enter a game prompt in Koschei Web Game Factory.</li>
        <li>Generate an HTML5 demo and review the live preview.</li>
        <li>Generate the Web3-ready package with item schemas and NFT metadata.</li>
        <li>Export artifacts for your own integration workflow.</li>
      </ol>
    </section>

    <section className="rounded bg-amber-100 p-4 text-sm">
      <h2 className="mb-1 text-base font-semibold">MVP safety boundary</h2>
      <p>{gameFactorySafetyCopy}</p>
    </section>

    <SupportCta />
  </main>;
}
