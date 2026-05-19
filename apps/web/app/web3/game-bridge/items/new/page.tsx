"use client";
import { useState } from "react";
import { gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";

const defaults = {
  item_key: "sword_of_koschei",
  name: "Sword of Koschei",
  item_type: "weapon",
  rarity: "legendary",
  image_uri: "",
  attributes: { attack: 99, durability: 500 }
};

export default function NewItemPage() {
  const [payload, setPayload] = useState(JSON.stringify(defaults, null, 2));
  const [result, setResult] = useState("");

  const submit = async () => {
    const res = await fetch("/api/game-bridge/items", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: payload
    });
    setResult(JSON.stringify(await res.json(), null, 2));
  };

  return <main className="mx-auto max-w-3xl space-y-3 p-6">
    <h1 className="text-2xl font-bold">Create Game Item Metadata</h1>
    <p className="text-sm text-gray-700">Prepare game item ownership records and metadata payloads for Godot integrations.</p>
    <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>
    <textarea className="min-h-72 w-full border p-2 font-mono text-xs" value={payload} onChange={(e) => setPayload(e.target.value)} />
    <button className="rounded bg-black px-3 py-2 text-white" onClick={submit}>Create Item</button>
    {result && <pre className="rounded bg-gray-100 p-3 text-xs">{result}</pre>}
  </main>;
}
