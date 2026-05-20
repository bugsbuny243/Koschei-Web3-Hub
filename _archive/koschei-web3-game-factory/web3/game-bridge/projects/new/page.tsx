"use client";
import { useState } from "react";

export default function NewProjectPage() {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [result, setResult] = useState("");
  const submit = async () => {
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "");
    const res = await fetch("/api/game-bridge/projects", { method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({ name, slug, description }) });
    setResult(JSON.stringify(await res.json(), null, 2));
  };
  return <main className="mx-auto max-w-3xl p-6 space-y-3"><h1 className="text-2xl font-bold">Create Game Project</h1>
    <input className="w-full border p-2" placeholder="Project name" value={name} onChange={(e)=>setName(e.target.value)} />
    <textarea className="w-full border p-2" placeholder="Description" value={description} onChange={(e)=>setDescription(e.target.value)} />
    <button className="rounded bg-black px-3 py-2 text-white" onClick={submit}>Create</button>
    {result && <pre className="rounded bg-gray-100 p-3 text-xs">{result}</pre>}
  </main>;
}
