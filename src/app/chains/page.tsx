"use client";

import { useState } from "react";
import { SiteHeader } from "@/components/SiteHeader";
import { ChainHealthCard, type Health } from "@/components/ChainHealthCard";

const chains = [
  { chain: "solana", network: "Solana Devnet" },
  { chain: "base", network: "Base Sepolia" },
  { chain: "arbitrum", network: "Arbitrum Sepolia" },
  { chain: "polygon", network: "Polygon Amoy" },
  { chain: "optimism", network: "Optimism Sepolia" },
  { chain: "ethereum", network: "Ethereum Sepolia" },
];

export default function Chains() {
  const [healthByChain, setHealthByChain] = useState<Record<string, Health>>({});
  const [checkingChains, setCheckingChains] = useState<string[]>([]);

  async function check(chain: string, network: string) {
    setCheckingChains((current) => [...new Set([...current, chain])]);
    try {
      const response = await fetch(`/api/web3/health?chain=${chain}`);
      const health = (await response.json()) as Health;
      setHealthByChain((current) => ({ ...current, [chain]: health }));
    } catch {
      setHealthByChain((current) => ({
        ...current,
        [chain]: { ok: false, status: "error", chain, network, provider: "Server-side RPC", error: "Health request failed. Please try again." },
      }));
    } finally {
      setCheckingChains((current) => current.filter((item) => item !== chain));
    }
  }

  async function checkAll() {
    await Promise.all(chains.map(({ chain, network }) => check(chain, network)));
  }

  const checkingAll = checkingChains.length === chains.length;

  return <main className="web3-page"><SiteHeader/><section className="mx-auto max-w-7xl px-5 py-12 sm:py-16 lg:px-8"><div className="flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-end"><div><p className="eyebrow">Provider diagnostics</p><h1 className="mt-4 text-3xl font-black text-white sm:text-4xl">ChainOps Dashboard</h1><p className="mt-4 max-w-3xl text-sm leading-7 text-slate-400">Run server-side health checks for supported test environments. Provider keys and full RPC URLs never enter the browser response.</p></div><button type="button" onClick={checkAll} disabled={checkingChains.length > 0} className="web3-button shrink-0">{checkingAll ? "Checking all…" : "Check all chains"}</button></div><div className="mt-9 grid gap-5 md:grid-cols-2 lg:grid-cols-3">{chains.map(({ chain, network }) => <ChainHealthCard key={chain} chain={chain} network={network} health={healthByChain[chain]} loading={checkingChains.includes(chain)} onCheck={() => check(chain, network)}/>)}</div></section></main>;
}
