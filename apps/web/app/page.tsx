import Link from "next/link";
import { SupportCta } from "@/components/support-cta";
import { gameFactorySafetyCopy } from "@/lib/game-factory";

export default function Home() {
  return <main className="mx-auto max-w-5xl space-y-6 p-6">
    <section className="rounded-xl border bg-gradient-to-r from-cyan-50 to-emerald-50 p-6">
      <h1 className="text-4xl font-bold">Koschei Web Game Factory + Web3 Bridge</h1>
      <p className="mt-3 text-gray-700">Prompt-to-playable HTML5 game demos with one-click Web3-ready package generation.</p>
      <p className="mt-3 rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link className="rounded bg-black px-4 py-2 text-white" href="/game-factory/new">Create Game</Link>
        <Link className="rounded border px-4 py-2" href="/web3/game-bridge">View Web3 Bridge</Link>
        <a className="rounded border border-emerald-700 px-4 py-2 text-emerald-700" href="https://www.shopier.com/TradeVisual/47208457" target="_blank">Support with 10 TL</a>
      </div>
    </section>
    <SupportCta />
  </main>;
}
