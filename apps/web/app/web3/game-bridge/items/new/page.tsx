"use client";

import { useMemo, useState } from "react";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";

type FormState = {
  itemName: string;
  description: string;
  rarity: string;
  imageUrl: string;
  attributes: string;
  targetChain: string;
};

const initialState: FormState = {
  itemName: "Sword of Koschei",
  description: "Ancient blade forged for dungeon runs.",
  rarity: "legendary",
  imageUrl: "",
  attributes: '{"attack":99,"durability":500}',
  targetChain: "arbitrum-sepolia"
};

export default function NewItemPage() {
  const [form, setForm] = useState<FormState>(initialState);

  const parsedAttributes = useMemo(() => {
    try {
      return JSON.parse(form.attributes || "{}");
    } catch {
      return { parse_error: "Attributes must be valid JSON" };
    }
  }, [form.attributes]);

  const metadataPreview = useMemo(() => ({
    name: form.itemName,
    description: form.description,
    image: form.imageUrl,
    target_chain: form.targetChain,
    attributes: [
      { trait_type: "rarity", value: form.rarity },
      ...Object.entries(parsedAttributes).map(([trait_type, value]) => ({ trait_type, value }))
    ]
  }), [form, parsedAttributes]);

  return <main className="mx-auto max-w-4xl space-y-4 p-6">
    <h1 className="text-3xl font-bold">Create Demo Item Metadata</h1>
    <p className="text-sm text-gray-700">No minting, no transaction, no wallet required.</p>
    <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>

    <section className="grid gap-3 md:grid-cols-2">
      <label className="space-y-1 text-sm">item name<input className="w-full rounded border p-2" value={form.itemName} onChange={(e) => setForm({ ...form, itemName: e.target.value })} /></label>
      <label className="space-y-1 text-sm">rarity<input className="w-full rounded border p-2" value={form.rarity} onChange={(e) => setForm({ ...form, rarity: e.target.value })} /></label>
      <label className="space-y-1 text-sm md:col-span-2">description<textarea className="w-full rounded border p-2" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></label>
      <label className="space-y-1 text-sm md:col-span-2">image URL<input className="w-full rounded border p-2" value={form.imageUrl} onChange={(e) => setForm({ ...form, imageUrl: e.target.value })} /></label>
      <label className="space-y-1 text-sm md:col-span-2">attributes<textarea className="min-h-24 w-full rounded border p-2 font-mono text-xs" value={form.attributes} onChange={(e) => setForm({ ...form, attributes: e.target.value })} /></label>
      <label className="space-y-1 text-sm">target chain<input className="w-full rounded border p-2" value={form.targetChain} onChange={(e) => setForm({ ...form, targetChain: e.target.value })} /></label>
    </section>

    <section>
      <h2 className="mb-2 text-xl font-semibold">NFT-compatible metadata JSON preview</h2>
      <pre className="overflow-x-auto rounded bg-gray-100 p-3 text-xs">{JSON.stringify(metadataPreview, null, 2)}</pre>
    </section>
  </main>;
}
