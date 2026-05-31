"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";

type Props = { mode: "login" | "signup" };

export function AuthForm({ mode }: Props) {
  const router = useRouter();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const isSignup = mode === "signup";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(""); setLoading(true);
    const data = new FormData(event.currentTarget);
    try {
      const response = await fetch(`/api/auth/${mode}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: data.get("email"), password: data.get("password") }) });
      const payload = await response.json() as { error?: string };
      if (!response.ok) throw new Error(payload.error || "Authentication failed.");
      router.push("/dashboard"); router.refresh();
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Authentication failed."); } finally { setLoading(false); }
  }

  return <form onSubmit={submit} className="glass-card mt-8 grid gap-5 p-6 sm:p-8">
    <label className="web3-label">Email<input className="web3-input" name="email" type="email" autoComplete="email" required /></label>
    <label className="web3-label">Password<input className="web3-input" name="password" type="password" autoComplete={isSignup ? "new-password" : "current-password"} minLength={8} required /></label>
    {error ? <p className="rounded-lg border border-red-400/30 bg-red-500/10 p-3 text-sm text-red-200">{error}</p> : null}
    <button className="web3-button" disabled={loading}>{loading ? "Please wait…" : isSignup ? "Create account" : "Sign in"}</button>
    <p className="text-sm text-slate-400">{isSignup ? "Already have an account?" : "New to Koschei?"} <Link className="font-bold text-blue-300 hover:text-white" href={isSignup ? "/login" : "/signup"}>{isSignup ? "Sign in" : "Get started"}</Link></p>
  </form>;
}
