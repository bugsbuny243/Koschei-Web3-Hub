"use client";

import { useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import { SiteHeader } from "@/components/SiteHeader";

const modes = [["metadata", "NFT metadata description"], ["description", "Token/project summary"], ["lore", "Game item lore"], ["pitch", "Ecosystem pitch"], ["launch", "Launch page copy"]];
const samplePrompt = "Koschei Arena is a Solana Web3 arena game concept. Draft content for the Shadow Reactor Blade, a collectible in-game weapon metadata concept with rarity, class, power, and element attributes. Keep claims verifiable and do not imply token deployment or investment returns.";

export default function Metadata() {
  const [mode, setMode] = useState("metadata");
  const [prompt, setPrompt] = useState("");
  const [output, setOutput] = useState("");
  const [fallback, setFallback] = useState(false);
  const [loading, setLoading] = useState(false);

  async function generate() { setLoading(true); try { const response = await fetch("/api/ai/web3-generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode, payload: { prompt } }) }); if (!response.ok) throw new Error("Generation failed."); const data = await response.json() as { text?: string; usedFallback?: boolean }; if (!data.text) throw new Error("Empty response."); setOutput(data.text); setFallback(Boolean(data.usedFallback)); } catch { setOutput("Fallback output: We could not reach the generator. Add verified project facts, intended utility, and ecosystem context, then review this draft before publishing."); setFallback(true); } finally { setLoading(false); } }

  return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-5xl px-5 py-16 lg:px-8"><p className="eyebrow">AI output studio</p><h1 className="mt-4 text-4xl font-black text-white">AI Metadata Studio</h1><div className="mt-9 grid gap-5 lg:grid-cols-2"><div className="glass-card p-6"><label className="web3-label">Mode<select value={mode} onChange={(event) => setMode(event.target.value)} className="web3-input">{modes.map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select></label><label className="web3-label mt-4">Project facts<textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} className="web3-input min-h-52" placeholder="Provide verified facts, intended utility and audience." /></label><div className="mt-4 flex flex-wrap gap-3"><button className="web3-button-secondary" type="button" onClick={() => setPrompt(samplePrompt)}>Use sample prompt</button><button className="web3-button" type="button" onClick={generate} disabled={loading}>{loading ? "Generating…" : "Generate safe copy"}</button></div></div><div className="glass-card p-6"><div className="flex flex-wrap items-center justify-between gap-3"><p className="eyebrow">Generated output</p>{output && <CopyButton text={output} label="Copy output" />}</div><p aria-live="polite" className="mt-5 whitespace-pre-wrap text-sm leading-7 text-slate-300">{loading ? "Generating safe copy…" : output || "Your editable output will appear here."}</p>{output && <p className="mt-5 text-xs font-bold text-slate-400">{fallback ? "Fallback output" : "AI output"} · Review before publishing.</p>}</div></div></section></main>;
}
