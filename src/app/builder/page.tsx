"use client";

import { FormEvent, useMemo, useState } from "react";
import { CopyButton } from "@/components/CopyButton";
import { JsonPreview } from "@/components/JsonPreview";
import { SiteHeader } from "@/components/SiteHeader";

type Form = { project: string; ecosystem: string; assetType: string; name: string; category: string; description: string; utility: string; attributes: string; externalUrl: string; imageUrl: string; riskNotes: string };
type Message = { kind: "success" | "error"; text: string } | null;

const initial: Form = { project: "", ecosystem: "Solana", assetType: "Game Item", name: "", category: "", description: "", utility: "", attributes: "", externalUrl: "", imageUrl: "", riskNotes: "" };
const sample: Form = { project: "Koschei Arena", ecosystem: "Solana", assetType: "Game Item", name: "Shadow Reactor Blade", category: "Weapon", description: "A concept game item for Koschei Arena.", utility: "in-game metadata concept", attributes: "rarity: legendary, class: shadow, power: 92, element: plasma", externalUrl: "", imageUrl: "", riskNotes: "Concept metadata only. Confirm artwork rights and in-game utility before publishing." };

function parseAttributes(value: string) {
  const entries = value.split(",").map((item) => item.trim()).filter(Boolean);
  if (entries.length && entries.every((item) => item.includes(":"))) return Object.fromEntries(entries.map((item) => { const [key, ...rest] = item.split(":"); return [key.trim(), rest.join(":").trim()]; }));
  return entries;
}

function StatusMessage({ message, className = "" }: { message: Message; className?: string }) {
  if (!message) return null;
  return <p role={message.kind === "error" ? "alert" : "status"} className={`${className} text-xs font-bold ${message.kind === "error" ? "text-rose-300" : "text-emerald-300"}`}>{message.text}</p>;
}

export default function Builder() {
  const [form, setForm] = useState(initial);
  const [saveMessage, setSaveMessage] = useState<Message>(null);
  const [downloadMessage, setDownloadMessage] = useState<Message>(null);
  const [aiMessage, setAiMessage] = useState<Message>(null);
  const [generating, setGenerating] = useState(false);
  const metadata = useMemo(() => ({ project: form.project, ecosystem: form.ecosystem, assetType: form.assetType, name: form.name, category: form.category, description: form.description, utility: form.utility, attributes: parseAttributes(form.attributes), external_url: form.externalUrl, image: form.imageUrl, riskNotes: form.riskNotes, notice: "Concept metadata only. No token or contract is created or deployed." }), [form]);

  function update(key: keyof Form, value: string) { setForm((current) => ({ ...current, [key]: value })); setSaveMessage(null); setDownloadMessage(null); }
  function fillSample() { setForm(sample); setSaveMessage(null); setDownloadMessage(null); setAiMessage(null); }
  function save(event: FormEvent) {
    event.preventDefault();
    try {
      let old: unknown[] = [];
      try { old = JSON.parse(localStorage.getItem("koschei_assets") || "[]") as unknown[]; if (!Array.isArray(old)) old = []; } catch { old = []; }
      localStorage.setItem("koschei_assets", JSON.stringify([...old, { ...metadata, createdAt: new Date().toISOString() }]));
      setSaveMessage({ kind: "success", text: "Saved to this browser's localStorage." });
    } catch {
      setSaveMessage({ kind: "error", text: "Could not save in this browser. Check localStorage availability and try again." });
    }
  }
  async function generate() {
    setGenerating(true);
    setAiMessage(null);
    try {
      const response = await fetch("/api/ai/web3-generate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode: "description", payload: form }) });
      if (!response.ok) throw new Error("AI request failed.");
      const data = await response.json() as { text?: unknown; usedFallback?: unknown };
      if (typeof data.text !== "string") throw new Error("AI response was invalid.");
      update("description", data.text);
      setAiMessage({ kind: "success", text: data.usedFallback === true ? "Fallback output" : "AI output" });
    } catch {
      setAiMessage({ kind: "error", text: "AI generation is unavailable right now. Your current description is safe; please try again." });
    } finally {
      setGenerating(false);
    }
  }
  function download() {
    try {
      const url = URL.createObjectURL(new Blob([JSON.stringify(metadata, null, 2)], { type: "application/json" }));
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `${form.name || "koschei-asset"}.json`;
      anchor.click();
      URL.revokeObjectURL(url);
      setDownloadMessage({ kind: "success", text: "JSON downloaded." });
    } catch {
      setDownloadMessage({ kind: "error", text: "Could not download JSON. Please try again." });
    }
  }

  return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-7xl px-5 py-12 sm:py-16 lg:px-8"><p className="eyebrow">No-code output studio</p><h1 className="mt-4 text-3xl font-black text-white sm:text-4xl">Web3 Asset Builder</h1><p className="mt-4 text-sm text-slate-400">Create metadata concepts only. No tokens, contracts or private keys are used.</p><div className="mt-9 grid gap-7 lg:grid-cols-2"><form onSubmit={save} className="glass-card grid min-w-0 gap-4 p-5 sm:grid-cols-2 sm:p-6">{([["Project name", "project"], ["Asset name", "name"], ["Category", "category"], ["External URL", "externalUrl"], ["Image URL", "imageUrl"]] as [string, keyof Form][]).map(([label, key]) => <label className="web3-label" key={key}>{label}<input className="web3-input" value={form[key]} onChange={(event) => update(key, event.target.value)} required={key === "project" || key === "name"} /></label>)}<label className="web3-label">Ecosystem<select className="web3-input" value={form.ecosystem} onChange={(event) => update("ecosystem", event.target.value)}>{["Solana", "Base", "Arbitrum", "Polygon", "Optimism", "Ethereum"].map((item) => <option key={item}>{item}</option>)}</select></label><label className="web3-label">Asset type<select className="web3-input" value={form.assetType} onChange={(event) => update("assetType", event.target.value)}>{["Game Item", "NFT Collection", "Token Concept", "Launch Page"].map((item) => <option key={item}>{item}</option>)}</select></label>{([["Description", "description"], ["Utility", "utility"], ["Attributes (comma separated key: value pairs)", "attributes"], ["Risk notes", "riskNotes"]] as [string, keyof Form][]).map(([label, key]) => <label className="web3-label sm:col-span-2" key={key}>{label}<textarea className="web3-input min-h-20" value={form[key]} onChange={(event) => update(key, event.target.value)} /></label>)}<div className="flex flex-wrap gap-3 sm:col-span-2"><button className="web3-button" type="submit">Save generated asset</button><button className="web3-button-secondary" type="button" onClick={fillSample}>Fill Sample Data</button><button className="web3-button-secondary" type="button" onClick={generate} disabled={generating}>{generating ? "Generating…" : "AI Generate Description"}</button></div><StatusMessage className="sm:col-span-2" message={saveMessage} /><StatusMessage className="sm:col-span-2" message={aiMessage} /></form><div className="min-w-0"><div className="mb-3 flex flex-wrap items-center gap-3"><CopyButton text={JSON.stringify(metadata, null, 2)} label="Copy JSON" successMessage="JSON copied." errorMessage="Could not copy JSON. Check clipboard permissions and try again." /><button type="button" onClick={download} className="web3-button-secondary">Download JSON</button><StatusMessage message={downloadMessage} /></div><JsonPreview value={metadata} /></div></div></section></main>;
}
