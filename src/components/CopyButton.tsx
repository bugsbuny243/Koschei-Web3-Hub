"use client";

import { useState } from "react";

type CopyStatus = "idle" | "success" | "error";

export function CopyButton({ text, label = "Copy", successMessage = "Copied to clipboard.", errorMessage = "Could not copy to clipboard. Please try again." }: { text: string; label?: string; successMessage?: string; errorMessage?: string }) {
  const [status, setStatus] = useState<CopyStatus>("idle");

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setStatus("success");
    } catch {
      setStatus("error");
    }
  }

  return <div className="flex flex-wrap items-center gap-2"><button type="button" onClick={copy} className="web3-button-secondary">{status === "success" ? "Copied" : label}</button>{status !== "idle" && <span role={status === "error" ? "alert" : "status"} className={`text-xs font-bold ${status === "error" ? "text-rose-300" : "text-emerald-300"}`}>{status === "error" ? errorMessage : successMessage}</span>}</div>;
}
