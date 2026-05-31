"use client";
import { useState } from "react";
export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) { const [copied, setCopied] = useState(false); return <button type="button" onClick={async () => { await navigator.clipboard.writeText(text); setCopied(true); window.setTimeout(() => setCopied(false), 1600); }} className="web3-button-secondary">{copied ? "Copied" : label}</button>; }
