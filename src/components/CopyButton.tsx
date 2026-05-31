"use client";

import { useState } from "react";

export function CopyButton({ text, label = "Copy", successMessage = "Copied to clipboard." }: { text: string; label?: string; successMessage?: string }) {
  const [copied, setCopied] = useState(false);

  return <div className="flex flex-wrap items-center gap-2"><button type="button" onClick={async () => { await navigator.clipboard.writeText(text); setCopied(true); window.setTimeout(() => setCopied(false), 1600); }} className="web3-button-secondary">{copied ? "Copied" : label}</button>{copied && <span role="status" className="text-xs font-bold text-emerald-300">{successMessage}</span>}</div>;
}
