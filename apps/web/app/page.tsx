"use client";

import { useState } from "react";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { SupportCta } from "@/components/support-cta";

export default function Home() {
  const [prompt, setPrompt] = useState("");
  const [result, setResult] = useState<string>("");

  const handleCreate = async () => {
    const res = await fetch("/api/agent/create", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt })
    });
    const data = await res.json();
    setResult(JSON.stringify(data, null, 2));
  };

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Koscei Bridge</h1>
        <ConnectButton />
      </div>

      <div className="rounded-xl border p-4 shadow-sm">
        <label className="mb-2 block text-lg font-semibold">Create your AI Agent</label>
        <textarea
          className="min-h-40 w-full rounded-lg border p-3"
          placeholder="Create your AI Agent..."
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
        />
        <button onClick={handleCreate} className="mt-3 rounded bg-black px-4 py-2 text-white">
          Create Agent
        </button>
      </div>

      {result && <pre className="overflow-auto rounded bg-gray-100 p-4 text-sm">{result}</pre>}

      <SupportCta />
    </main>
  );
}
