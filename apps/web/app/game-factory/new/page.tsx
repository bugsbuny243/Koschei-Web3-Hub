"use client";
import { useState } from "react";

export default function NewGameFactoryProjectPage() {
  const [name, setName] = useState("");
  const [prompt, setPrompt] = useState("");
  const [result, setResult] = useState("");
  const create = async () => {
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "");
    const res = await fetch("/api/game-factory/projects", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, slug, prompt }) });
    setResult(JSON.stringify(await res.json(), null, 2));
  };
  return <main className="mx-auto max-w-3xl space-y-4 p-6"><h1 className="text-3xl font-bold">New Game Factory Project</h1><input className="w-full rounded border p-2" placeholder="Project name" value={name} onChange={(e) => setName(e.target.value)} /><textarea className="min-h-40 w-full rounded border p-2" placeholder="Describe the game to generate" value={prompt} onChange={(e) => setPrompt(e.target.value)} /><button className="rounded bg-black px-4 py-2 text-white" onClick={create}>Create</button>{result && <pre className="overflow-auto rounded bg-gray-100 p-3 text-xs">{result}</pre>}</main>;
}
