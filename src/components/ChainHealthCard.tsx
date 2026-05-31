export type Health = {
  ok: boolean;
  status: "online" | "error" | "not-configured";
  chain: string;
  network: string;
  provider: string;
  error?: string;
};

export function ChainHealthCard({ chain, network, health, loading, onCheck }: { chain: string; network: string; health?: Health; loading: boolean; onCheck: () => void }) {
  const status = loading ? "Checking…" : health?.status === "online" ? "Online" : health?.status === "not-configured" ? "Not configured" : health ? "Error" : "Not checked";
  const statusColor = loading ? "text-blue-300" : health?.status === "online" ? "text-emerald-300" : health?.status === "not-configured" ? "text-amber-300" : health ? "text-rose-300" : "text-slate-400";
  const dotColor = loading ? "bg-blue-400" : health?.status === "online" ? "bg-emerald-400" : health?.status === "not-configured" ? "bg-amber-400" : health ? "bg-rose-400" : "bg-slate-600";

  return <article className="glass-card min-w-0 p-5 sm:p-6"><div className="flex items-start justify-between gap-4"><div className="min-w-0"><p className="eyebrow">{chain}</p><h3 className="mt-2 text-lg font-bold text-white">{network}</h3></div><span className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${dotColor}`} /></div><dl className="mt-5 grid gap-3 text-xs"><div className="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-2"><dt className="font-bold text-slate-500">Chain</dt><dd className="web3-break-long text-slate-300">{health?.chain || chain}</dd></div><div className="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-2"><dt className="font-bold text-slate-500">Network</dt><dd className="web3-break-long text-slate-300">{health?.network || network}</dd></div><div className="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-2"><dt className="font-bold text-slate-500">Provider</dt><dd className="web3-break-long text-slate-300">{health?.provider || "Server-side RPC"}</dd></div><div className="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-2"><dt className="font-bold text-slate-500">Status</dt><dd className={`font-black ${statusColor}`}>{status}</dd></div></dl>{health && !health.ok && <p className="web3-break-long mt-4 rounded-lg border border-white/10 bg-black/10 p-3 text-xs leading-5 text-slate-300">{health.error || "The provider could not be reached. Please try again."}</p>}<button type="button" onClick={onCheck} disabled={loading} className="web3-button-secondary mt-5 w-full sm:w-auto">{loading ? "Checking…" : "Check health"}</button></article>;
}
