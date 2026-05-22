"use client";

import { FormEvent, useMemo, useState } from "react";

type Message = { role: "user" | "assistant"; text: string; type?: string; mediaUrl?: string };

function detectLocalType(prompt: string) {
  const lower = prompt.toLowerCase();
  if (/(write code|typescript|python|kod|function|api)/i.test(lower)) return "code";
  if (/(image|draw|logo|resim|görsel)/i.test(lower)) return "image";
  if (/(video|cinematic|clip|kısa video)/i.test(lower)) return "video";
  return "chat";
}

export default function HomePage() {
  const [prompt, setPrompt] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(false);
  const [credits, setCredits] = useState<number | null>(null);

  const token = useMemo(() => (typeof window !== "undefined" ? localStorage.getItem("koschei_token") : null), []);

  async function sendPrompt(e: FormEvent) {
    e.preventDefault();
    if (!prompt.trim()) return;
    const localType = detectLocalType(prompt);
    const userMessage: Message = { role: "user", text: prompt, type: localType };
    setMessages((m) => [...m, userMessage]);
    setLoading(true);
    setPrompt("");

    try {
      const res = await fetch("/api/ai", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token ?? ""}` },
        body: JSON.stringify({ prompt, type: localType }),
      });
      if (!res.ok || !res.body) {
        const errorText = await res.text();
        throw new Error(errorText || "Request failed");
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let assistantText = "";
      let metadataSeen = false;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value);
        if (!metadataSeen) {
          const lines = chunk.split("\n");
          try {
            const metadata = JSON.parse(lines[0] ?? "{}");
            if (typeof metadata.credits === "number") setCredits(metadata.credits);
            metadataSeen = true;
            assistantText += lines.slice(1).join("\n");
          } catch {
            assistantText += chunk;
          }
        } else {
          assistantText += chunk;
        }
        setMessages((prev) => [...prev.filter((x) => x.role === "user"), { role: "assistant", text: assistantText, type: localType }]);
      }
    } catch (error) {
      setMessages((m) => [...m, { role: "assistant", text: error instanceof Error ? error.message : "Unknown error" }]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="container page-stack">
      <section className="hero">
        <p className="eyebrow">KOSCHEI AI SUPER-APP</p>
        <h1>Talk in English or Turkish — code, chat, image & video in one place.</h1>
        <p>Automatic model routing: Qwen for code, Llama for chat, FLUX for images and Veo for video.</p>
        <p><strong>Remaining credits:</strong> {credits ?? "-"}</p>
      </section>

      <section className="card">
        <form onSubmit={sendPrompt} className="form-grid" style={{ gridTemplateColumns: "1fr auto" }}>
          <input value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder="Type in Turkish or English..." />
          <button className="btn btn-primary" disabled={loading}>{loading ? "Thinking..." : "Send"}</button>
        </form>
      </section>

      <section className="card" style={{ display: "grid", gap: 12 }}>
        {messages.map((msg, i) => (
          <article key={i} style={{ background: msg.role === "user" ? "#152635" : "#0f1c2b", color: "#f8fafc", padding: 12, borderRadius: 10 }}>
            <p className="eyebrow" style={{ color: "#93c5fd" }}>{msg.role} {msg.type ? `• ${msg.type}` : ""}</p>
            {msg.type === "code" ? <pre style={{ whiteSpace: "pre-wrap" }}>{msg.text}</pre> : <p style={{ whiteSpace: "pre-wrap" }}>{msg.text}</p>}
          </article>
        ))}
      </section>
    </div>
  );
}
