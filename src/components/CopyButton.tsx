"use client";
import { useState } from "react";
export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) { const [status, setStatus] = useState<"idle"|"copied"|"failed">("idle"); return <button type="button" onClick={async () => { try { await navigator.clipboard.writeText(text); setStatus("copied"); } catch { setStatus("failed"); } window.setTimeout(() => setStatus("idle"), 1600); }} className="web3-button-secondary">{status==="copied" ? "Copied" : status==="failed" ? "Copy failed" : label}</button>; }
