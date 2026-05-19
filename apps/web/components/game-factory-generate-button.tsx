"use client";
import { useRouter } from "next/navigation";
import { useState } from "react";

export function GameFactoryGenerateButton({ id, kind }: { id: string; kind: "preview" | "web3" }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  const onClick = async () => {
    setLoading(true); setError(null);
    try {
      const url = kind === "preview" ? `/api/game-factory/projects/${id}/generate` : `/api/game-factory/projects/${id}/web3-package`;
      const r = await fetch(url, { method: "POST" });
      if (!r.ok) throw new Error((await r.json())?.error || "request_failed");
      router.refresh();
    } catch (e) { setError(e instanceof Error ? e.message : "request_failed"); }
    finally { setLoading(false); }
  };

  return <div className="space-y-2"><button onClick={onClick} className="rounded border px-3 py-2" disabled={loading}>{loading ? "Generating..." : kind === "preview" ? "Generate Game Preview" : "Generate Web3 Package"}</button>{error && <p className="text-sm text-red-600">{error}</p>}</div>;
}
