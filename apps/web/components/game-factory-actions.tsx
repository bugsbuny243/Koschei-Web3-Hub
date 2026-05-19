"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export function GameFactoryActions({ id }: { id: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState<"generate"|"web3"|null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = async (kind: "generate"|"web3") => {
    setError(null); setBusy(kind);
    try {
      const url = kind === "generate" ? `/api/game-factory/projects/${id}/generate` : `/api/game-factory/projects/${id}/web3-package`;
      const r = await fetch(url, { method: "POST" });
      if (!r.ok) throw new Error((await r.json())?.error || "request_failed");
      router.refresh();
    } catch (e) { setError(e instanceof Error ? e.message : "request_failed"); }
    finally { setBusy(null); }
  };

  return <div className="space-y-3"><div className="flex flex-wrap gap-3">
    <button className="rounded border px-3 py-2" disabled={!!busy} onClick={() => run("generate")}>{busy === "generate" ? "Generating..." : "Generate / Regenerate Game"}</button>
    <button className="rounded border px-3 py-2" disabled={!!busy} onClick={() => run("web3")}>{busy === "web3" ? "Generating..." : "Generate Web3 Package"}</button>
    <a href={`/game-factory/projects/${id}/preview`} className="rounded border px-3 py-2">Open Preview</a>
    <a href={`/game-factory/projects/${id}/web3`} className="rounded border px-3 py-2">Open Web3 Package</a>
  </div>{error && <p className="rounded border border-red-300 bg-red-50 p-2 text-sm text-red-700">{error}</p>}</div>;
}
