import Link from "next/link";
import { SiteHeader } from "@/components/SiteHeader";
import { Web3Card } from "@/components/Web3Card";
import { Web3PricingSection } from "@/components/Web3PricingSection";

const modules = [
  ["Web3 Hub Dashboard", "Open the central workspace for Web3 builder tools and ecosystem outputs.", "/hub"],
  ["Game Asset Builder", "Structure portable game items and NFT-ready asset metadata.", "/builder"],
  ["AI Metadata Studio", "Generate clear asset, project, lore, pitch and launch copy.", "/metadata"],
  ["Risk & Trust Scanner", "Turn project signals into a transparent risk checklist.", "/risk"],
  ["ChainOps Dashboard", "Check supported testnet RPC connectivity without exposing keys.", "/chains"],
  ["Ecosystem Layer", "Explore the ecosystem growth layer for builders, chains and communities.", "/ecosystem"],
  ["Developer Docs", "Review safe architecture, metadata schemas and integration guidance.", "/docs"],
  ["Admin / Intake", "Open the local MVP intake and feature-flag administration workspace.", "/admin"],
];

const footerLinks = [
  ["Hub", "/hub"],
  ["Builder", "/builder"],
  ["Metadata", "/metadata"],
  ["Risk", "/risk"],
  ["Chains", "/chains"],
  ["Ecosystem", "/ecosystem"],
  ["Docs", "/docs"],
  ["Admin", "/admin"],
];

export default function Home() {
  return (
    <main className="web3-page">
      <SiteHeader />
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_70%_20%,rgba(37,99,235,.22),transparent_34%),radial-gradient(circle_at_20%_55%,rgba(124,58,237,.15),transparent_30%)]" />
        <div className="relative mx-auto max-w-7xl px-5 py-24 lg:px-8 lg:py-32">
          <p className="eyebrow">Web3 builder infrastructure</p>
          <h1 className="mt-6 max-w-5xl text-4xl font-black tracking-tight text-white sm:text-7xl">
            Koschei <span className="bg-gradient-to-r from-blue-400 to-violet-400 bg-clip-text text-transparent">Web3 Hub</span>
          </h1>
          <p className="mt-7 max-w-3xl text-lg leading-8 text-slate-300">
            AI-powered Web3 operating layer for builders, game assets, metadata, risk transparency and ChainOps.
          </p>
          <div className="mt-9 flex flex-wrap gap-3">
            <Link href="/hub" className="web3-button">Open Web3 Hub →</Link>
            <Link href="/builder" className="web3-button-secondary">Start Building →</Link>
            <Link href="/chains" className="web3-button-secondary">Check Chains →</Link>
          </div>
          <div className="mt-14 grid gap-3 text-xs font-bold text-slate-400 sm:grid-cols-3">
            <span>✓ No custody</span>
            <span>✓ No private keys</span>
            <span>✓ No token trading</span>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-5 py-20 lg:px-8">
        <div className="grid gap-5 lg:grid-cols-2">
          <Web3Card title="Web3 builder problem" description="Web3 teams often navigate fragmented metadata, unclear trust signals, disconnected chain tooling and launch materials that are difficult to standardize." />
          <Web3Card title="Koschei solution" description="Koschei combines builder tooling, structured outputs, infrastructure health checks and risk transparency in one safety-first operating layer." />
        </div>
      </section>

      <section className="border-y border-white/10 bg-slate-950/40">
        <div className="mx-auto max-w-7xl px-5 py-20 lg:px-8">
          <p className="eyebrow">Core modules</p>
          <h2 className="mt-3 text-3xl font-black text-white">One serious layer for the builder lifecycle.</h2>
          <div className="mt-8 grid gap-5 md:grid-cols-2 lg:grid-cols-4">
            {modules.map(([title, description, href]) => <Web3Card key={title} title={title} description={description} status="Live MVP" href={href} />)}
          </div>
        </div>
      </section>

      <section className="mx-auto grid max-w-7xl gap-5 px-5 py-20 lg:grid-cols-2 lg:px-8">
        <Web3Card title="Safety-first architecture" description="Koschei is designed for builder-ready outputs and transparent checks. API secrets remain server-side, while builders retain control of their own assets and workflows." />
        <Web3Card title="ChainOps / Alchemy layer" description="Check supported RPC connectivity for Solana, Base, Arbitrum, Polygon, Optimism and Ethereum test environments through a server-side Alchemy layer without exposing provider credentials." />
      </section>

      <section className="mx-auto max-w-7xl px-5 pb-20 lg:px-8">
        <div className="glass-card p-8">
          <p className="eyebrow">No custody / no private keys / no token trading</p>
          <h2 className="mt-3 text-3xl font-black text-white">Built for outputs, not asset control.</h2>
          <p className="mt-4 max-w-3xl leading-7 text-slate-300">
            Koschei does not custody funds, request private keys or enable token trading. The Hub focuses on metadata, game assets, risk transparency and developer-ready ChainOps information.
          </p>
        </div>
      </section>

      <Web3PricingSection />

      <section className="border-t border-white/10 bg-blue-600/10">
        <div className="mx-auto max-w-7xl px-5 py-20 text-center lg:px-8">
          <p className="eyebrow">Build with confidence</p>
          <h2 className="mt-4 text-3xl font-black text-white sm:text-4xl">Build Web3 outputs that ecosystems can trust.</h2>
          <div className="mt-7 flex flex-wrap justify-center gap-3">
            <Link href="/hub" className="web3-button">Open Web3 Hub →</Link>
            <Link href="/builder" className="web3-button-secondary">Start Building →</Link>
            <Link href="/chains" className="web3-button-secondary">Check Chains →</Link>
          </div>
        </div>
      </section>

      <footer className="border-t border-white/10 bg-slate-950/70 px-5 py-8 text-sm font-semibold text-slate-400">
        <nav aria-label="Footer navigation" className="mx-auto flex max-w-7xl flex-wrap justify-center gap-x-5 gap-y-3">
          {footerLinks.map(([label, href]) => <Link key={href} href={href} className="hover:text-white">{label}</Link>)}
        </nav>
        <p className="mt-5 text-center">No custody. No private keys. No token trading. Builder-ready Web3 outputs only.</p>
      </footer>
    </main>
  );
}
