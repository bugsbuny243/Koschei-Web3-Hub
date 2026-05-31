"use client";

import { useState } from "react";

type Health = { ok: boolean; chain: string; network: string; provider?: string; result?: unknown; error?: string };
type DisplayStatus = "Online" | "Error" | "Not configured" | "Not checked";

export function ChainHealthCard({ chain, network }: { chain: string; network: string }) {
  const [health, setHealth] = useState<Health | null>(null);
  const [loading, setLoading] = useState(false);
  async function check() { setLoading(true); try { const response = await fetch(`/api/web3/health?chain=${chain}`); const data = await response.json() as Health; setHealth(response.ok ? data : { ...data, ok: false, chain, network, error: data.error || "Health check failed." }); } catch { setHealth({ ok: false, chain, network, error: "Health request failed. Try again shortly." }); } finally { setLoading(false); } }
  const status: DisplayStatus = !health ? "Not checked" : health.ok ? "Online" : health.error?.toLowerCase().includes("not configured") ? "Not configured" : "Error";
  const tone = status === "Online" ? "text-emerald-300" : status === "Not checked" ? "text-slate-400" : status === "Not configured" ? "text-amber-300" : "text-rose-300";
  const dot = status === "Online" ? "bg-emerald-400" : status === "Not checked" ? "bg-slate-600" : status === "Not configured" ? "bg-amber-400" : "bg-rose-400";
  return <article className="glass-card p-6"><div className="flex items-start justify-between gap-4"><div><p className="eyebrow">{chain}</p><h3 className="mt-2 text-lg font-bold text-white">{network}</h3></div><span className={`h-2.5 w-2.5 shrink-0 rounded-full ${dot}`} /></div><p className={`mt-4 text-sm font-black ${tone}`}>{loading ? "Checking…" : status}</p><p className="mt-2 min-h-10 text-xs leading-5 text-slate-400">{health ? health.ok ? "Server-side RPC connectivity check succeeded." : health.error : "Run a server-side RPC connectivity check."}</p><button onClick={check} disabled={loading} className="web3-button-secondary mt-4">{loading ? "Checking…" : "Check health"}</button></article>;
}
