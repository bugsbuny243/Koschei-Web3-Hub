"use client";

import { useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import { SiteHeader } from "@/components/SiteHeader";

const modes = [["metadata", "NFT metadata description"], ["description", "Token/project summary"], ["lore", "Game item lore"], ["pitch", "Ecosystem pitch"], ["launch", "Launch page copy"]];
const samplePrompt = "Project: Koschei Arena. Ecosystem: Solana. Asset type: Game Item. Asset name: Shadow Reactor Blade. Category: Weapon. Utility: in-game metadata concept. Attributes: rarity legendary, class shadow, power 92, element plasma. Write concise metadata copy without investment claims.";

export default function Metadata() {
  const [mode, setMode] = useState("metadata");
  const [prompt, setPrompt] = useState("");
  const [output, setOutput] = useState("");
  const [fallback, setFallback] = useState(false);
  const [loading, setLoading] = useState(false);

  async function generate() { setLoading(true); try { const response = await fetch("/api/ai/web3-generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode, payload: { prompt } }) }); const data = await response.json() as { text: string; usedFallback: boolean }; setOutput(data.text); setFallback(data.usedFallback); } finally { setLoading(false); } }

  return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-5xl px-5 py-12 sm:py-16 lg:px-8"><p className="eyebrow">AI output studio</p><h1 className="mt-4 text-3xl font-black text-white sm:text-4xl">AI Metadata Studio</h1><div className="mt-9 grid gap-5 lg:grid-cols-2"><div className="glass-card min-w-0 p-5 sm:p-6"><label className="web3-label">Mode<select value={mode} onChange={(event) => setMode(event.target.value)} className="web3-input">{modes.map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select></label><label className="web3-label mt-4">Project facts<textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} className="web3-input min-h-52" placeholder="Provide verified facts, intended utility and audience." /></label><div className="mt-4 flex flex-wrap gap-3"><button className="web3-button" type="button" onClick={generate} disabled={loading}>{loading ? "Generating…" : "Generate safe copy"}</button><button className="web3-button-secondary" type="button" onClick={() => setPrompt(samplePrompt)}>Fill Sample Prompt</button></div></div><div className="glass-card min-w-0 p-5 sm:p-6" aria-live="polite"><div className="flex flex-wrap items-center justify-between gap-3"><p className="eyebrow">{output ? (fallback ? "Fallback output" : "AI output") : "Generated output"}</p>{output && <CopyButton text={output} label="Copy Output" successMessage="Output copied." />}</div><p className="web3-break-long mt-5 whitespace-pre-wrap text-sm leading-7 text-slate-300">{loading ? "Generating safe copy…" : output || "Your editable output will appear here."}</p>{output && !loading && <p className="mt-5 text-xs text-slate-500">{fallback ? "Together AI was unavailable, so a deterministic fallback draft was used." : "Together AI generated this draft. Review before publishing."}</p>}</div></div></section></main>;
}
