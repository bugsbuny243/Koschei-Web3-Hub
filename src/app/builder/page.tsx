"use client";

import { FormEvent, useMemo, useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import { JsonPreview } from "@/components/JsonPreview";
import { SiteHeader } from "@/components/SiteHeader";

type Form = { project: string; ecosystem: string; assetType: string; name: string; category: string; description: string; utility: string; attributes: string; externalUrl: string; imageUrl: string; riskNotes: string };
const initial: Form = { project: "", ecosystem: "Solana", assetType: "Game Item", name: "", category: "", description: "", utility: "", attributes: "", externalUrl: "", imageUrl: "", riskNotes: "" };
const sample: Form = { project: "Koschei Arena", ecosystem: "Solana", assetType: "Game Item", name: "Shadow Reactor Blade", category: "Weapon", description: "A concept game item for Koschei Arena.", utility: "in-game metadata concept", attributes: "rarity: legendary, class: shadow, power: 92, element: plasma", externalUrl: "", imageUrl: "", riskNotes: "Concept metadata only. Confirm artwork rights and in-game utility before publishing." };

function parseAttributes(value: string) {
  const entries = value.split(",").map((item) => item.trim()).filter(Boolean);
  if (entries.length && entries.every((item) => item.includes(":"))) return Object.fromEntries(entries.map((item) => { const [key, ...rest] = item.split(":"); return [key.trim(), rest.join(":").trim()]; }));
  return entries;
}

export default function Builder() {
  const [form, setForm] = useState(initial);
  const [saved, setSaved] = useState(false);
  const [downloaded, setDownloaded] = useState(false);
  const [generating, setGenerating] = useState(false);
  const metadata = useMemo(() => ({ project: form.project, ecosystem: form.ecosystem, assetType: form.assetType, name: form.name, category: form.category, description: form.description, utility: form.utility, attributes: parseAttributes(form.attributes), external_url: form.externalUrl, image: form.imageUrl, riskNotes: form.riskNotes, notice: "Concept metadata only. No token or contract is created or deployed." }), [form]);

  function update(key: keyof Form, value: string) { setForm((current) => ({ ...current, [key]: value })); setSaved(false); setDownloaded(false); }
  function fillSample() { setForm(sample); setSaved(false); setDownloaded(false); }
  function save(event: FormEvent) { event.preventDefault(); let old: unknown[] = []; try { old = JSON.parse(localStorage.getItem("koschei_assets") || "[]") as unknown[]; if (!Array.isArray(old)) old = []; } catch { old = []; } localStorage.setItem("koschei_assets", JSON.stringify([...old, { ...metadata, createdAt: new Date().toISOString() }])); setSaved(true); }
  async function generate() { setGenerating(true); try { const response = await fetch("/api/ai/web3-generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode: "description", payload: form }) }); const data = await response.json() as { text: string }; update("description", data.text); } finally { setGenerating(false); } }
  function download() { const url = URL.createObjectURL(new Blob([JSON.stringify(metadata, null, 2)], { type: "application/json" })); const anchor = document.createElement("a"); anchor.href = url; anchor.download = `${form.name || "koschei-asset"}.json`; anchor.click(); URL.revokeObjectURL(url); setDownloaded(true); window.setTimeout(() => setDownloaded(false), 1600); }

  return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-7xl px-5 py-12 sm:py-16 lg:px-8"><p className="eyebrow">No-code output studio</p><h1 className="mt-4 text-3xl font-black text-white sm:text-4xl">Web3 Asset Builder</h1><p className="mt-4 text-sm text-slate-400">Create metadata concepts only. No tokens, contracts or private keys are used.</p><div className="mt-9 grid gap-7 lg:grid-cols-2"><form onSubmit={save} className="glass-card grid min-w-0 gap-4 p-5 sm:grid-cols-2 sm:p-6">{([["Project name", "project"], ["Asset name", "name"], ["Category", "category"], ["External URL", "externalUrl"], ["Image URL", "imageUrl"]] as [string, keyof Form][]).map(([label, key]) => <label className="web3-label" key={key}>{label}<input className="web3-input" value={form[key]} onChange={(event) => update(key, event.target.value)} required={key === "project" || key === "name"} /></label>)}<label className="web3-label">Ecosystem<select className="web3-input" value={form.ecosystem} onChange={(event) => update("ecosystem", event.target.value)}>{["Solana", "Base", "Arbitrum", "Polygon", "Optimism", "Ethereum"].map((item) => <option key={item}>{item}</option>)}</select></label><label className="web3-label">Asset type<select className="web3-input" value={form.assetType} onChange={(event) => update("assetType", event.target.value)}>{["Game Item", "NFT Collection", "Token Concept", "Launch Page"].map((item) => <option key={item}>{item}</option>)}</select></label>{([["Description", "description"], ["Utility", "utility"], ["Attributes (comma separated key: value pairs)", "attributes"], ["Risk notes", "riskNotes"]] as [string, keyof Form][]).map(([label, key]) => <label className="web3-label sm:col-span-2" key={key}>{label}<textarea className="web3-input min-h-20" value={form[key]} onChange={(event) => update(key, event.target.value)} /></label>)}<div className="flex flex-wrap gap-3 sm:col-span-2"><button className="web3-button" type="submit">Save generated asset</button><button className="web3-button-secondary" type="button" onClick={fillSample}>Fill Sample Data</button><button className="web3-button-secondary" type="button" onClick={generate} disabled={generating}>{generating ? "Generating…" : "AI Generate Description"}</button></div>{saved && <p role="status" className="text-xs font-bold text-emerald-300 sm:col-span-2">Saved to this browser&apos;s localStorage.</p>}</form><div className="min-w-0"><div className="mb-3 flex flex-wrap gap-3"><CopyButton text={JSON.stringify(metadata, null, 2)} label="Copy JSON" successMessage="JSON copied." /><button type="button" onClick={download} className="web3-button-secondary">Download JSON</button>{downloaded && <span role="status" className="self-center text-xs font-bold text-emerald-300">JSON downloaded.</span>}</div><JsonPreview value={metadata} /></div></div></section></main>;
}
