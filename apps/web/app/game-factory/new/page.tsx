"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export default function Page() {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [prompt, setPrompt] = useState("");
  const [genre, setGenre] = useState("");
  const [style, setStyle] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setError(null);
    if (!prompt.trim()) {
      setError("Prompt is required.");
      return;
    }
    setLoading(true);
    try {
      const createRes = await fetch("/api/game-factory/projects", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, prompt, genre, style, target_chain: "arbitrum-sepolia" })
      });
      const createJson = await createRes.json();
      if (!createRes.ok || !createJson?.project?.id) throw new Error(createJson?.error || "failed_to_create_project");
      const id = createJson.project.id as string;

      const genRes = await fetch(`/api/game-factory/projects/${id}/generate`, { method: "POST" });
      if (!genRes.ok) throw new Error((await genRes.json())?.error || "failed_to_generate");

      try {
        const web3Res = await fetch(`/api/game-factory/projects/${id}/web3-package`, { method: "POST" });
        if (!web3Res.ok) {
          const web3Json = await web3Res.json().catch(() => null);
          console.warn("Web3 package generation failed, continuing to preview", web3Json?.error || web3Json?.detail || web3Res.statusText);
        }
      } catch (e) {
        console.warn("Web3 package generation failed, continuing to preview", e);
      }

      router.push(`/game-factory/projects/${id}/preview`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unexpected error.");
    } finally {
      setLoading(false);
    }
  };

  return <main className="mx-auto max-w-3xl space-y-3 p-6"><h1 className="text-3xl font-bold">Create Game Project</h1>
    <input className="w-full rounded border p-2" placeholder="Game title (optional)" value={title} onChange={e=>setTitle(e.target.value)} disabled={loading}/>
    <textarea className="min-h-40 w-full rounded border p-2" placeholder="Prompt (required)" value={prompt} onChange={e=>setPrompt(e.target.value)} disabled={loading}/>
    <input className="w-full rounded border p-2" placeholder="Genre (optional)" value={genre} onChange={e=>setGenre(e.target.value)} disabled={loading}/>
    <input className="w-full rounded border p-2" placeholder="Style (optional)" value={style} onChange={e=>setStyle(e.target.value)} disabled={loading}/>
    <input className="w-full rounded border p-2 bg-gray-50" value="arbitrum-sepolia" readOnly/>
    <button className="rounded bg-black px-4 py-2 text-white disabled:opacity-60" onClick={submit} disabled={loading}>{loading ? "Generating project and preview..." : "Create Project"}</button>
    {error && <p className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-700">{error}</p>}
  </main>;
}
