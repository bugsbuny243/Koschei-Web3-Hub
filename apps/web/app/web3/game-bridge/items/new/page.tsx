"use client";
import { useState } from "react";

const defaults = { item_key: "sword_of_koschei", name: "Sword of Koschei", item_type: "weapon", rarity: "legendary", image_uri: "", attributes: { attack: 99, durability: 500 } };

export default function NewItemPage() {
  const [payload, setPayload] = useState(JSON.stringify(defaults, null, 2));
  const [result, setResult] = useState("");
  const submit = async () => {
    const res = await fetch("/api/game-bridge/items", { method: "POST", headers: {"Content-Type": "application/json"}, body: payload });
    setResult(JSON.stringify(await res.json(), null, 2));
  };
  return <main className="mx-auto max-w-3xl p-6 space-y-3"><h1 className="text-2xl font-bold">Create Game Item</h1>
    <textarea className="w-full min-h-72 border p-2 font-mono text-xs" value={payload} onChange={(e)=>setPayload(e.target.value)} />
    <button className="rounded bg-black px-3 py-2 text-white" onClick={submit}>Create Item</button>
    {result && <pre className="rounded bg-gray-100 p-3 text-xs">{result}</pre>}
  </main>;
}
