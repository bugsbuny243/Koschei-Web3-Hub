"use client";

import { FormEvent, useEffect, useState } from "react";
import { FeatureFlagBadge } from "@/components/FeatureFlagBadge";

const chainFlags: Record<string, string> = { FEATURE_SOLANA: "Solana", FEATURE_BASE: "Base", FEATURE_ARBITRUM: "Arbitrum", FEATURE_POLYGON: "Polygon", FEATURE_OPTIMISM: "Optimism", FEATURE_ETHEREUM: "Ethereum" };

export function AdminDashboard({ flags }: { flags: Record<string, boolean> }) {
  const [loggedIn, setLoggedIn] = useState(false);
  const [count, setCount] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => { const timeout = window.setTimeout(() => { setLoggedIn(sessionStorage.getItem("koschei_admin_session") === "active"); try { setCount((JSON.parse(localStorage.getItem("koschei_assets") || "[]") as unknown[]).length); } catch { setCount(0); } }, 0); return () => window.clearTimeout(timeout); }, []);
  async function login(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setError(""); const data = new FormData(event.currentTarget); try { const response = await fetch("/api/admin/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: data.get("email"), password: data.get("password") }) }); if (!response.ok) throw new Error("Login failed."); sessionStorage.setItem("koschei_admin_session", "active"); setLoggedIn(true); } catch { setError("Login failed. Check your admin email and password, or confirm that admin credentials are configured."); } }
  function logout() { sessionStorage.removeItem("koschei_admin_session"); setLoggedIn(false); setError(""); }
  if (!loggedIn) return <form onSubmit={login} className="glass-card mx-auto mt-9 max-w-md space-y-4 p-6"><label className="web3-label">Email<input name="email" type="email" className="web3-input" required /></label><label className="web3-label">Password<input name="password" type="password" className="web3-input" required /></label><button className="web3-button">Sign in</button>{error && <p role="alert" className="text-xs leading-5 text-rose-300">{error}</p>}</form>;
  const enabledChains = Object.entries(chainFlags).filter(([flag]) => flags[flag]).map(([, label]) => label);
  const enabledFlags = Object.entries(flags).filter(([, enabled]) => enabled);
  return <div className="mt-9"><div className="flex justify-end"><button type="button" className="web3-button-secondary" onClick={logout}>Log out</button></div><div className="mt-5 grid gap-5 md:grid-cols-4">{[["Generated assets", String(count)], ["Waitlist", "Placeholder"], ["Ecosystem leads", "Placeholder"], ["Project intake", "Placeholder"]].map(([label, value]) => <div className="glass-card p-5" key={label}><p className="text-xs font-bold text-slate-400">{label}</p><p className="mt-3 text-xl font-black text-white">{value}</p></div>)}</div><div className="mt-6 grid gap-6 lg:grid-cols-2"><div className="glass-card p-6"><h2 className="font-bold text-white">Enabled feature flags</h2><div className="mt-4 flex flex-wrap gap-2">{enabledFlags.length ? enabledFlags.map(([label]) => <FeatureFlagBadge key={label} label={label} enabled />) : <p className="text-sm text-slate-400">No feature flags enabled.</p>}</div></div><div className="glass-card p-6"><h2 className="font-bold text-white">Enabled chains</h2><p className="mt-4 text-sm leading-6 text-slate-300">{enabledChains.length ? enabledChains.join(" · ") : "No chains enabled. Configure feature flags on the server."}</p><h3 className="mt-6 text-sm font-bold text-white">Basic project intake</h3><ul className="mt-3 space-y-2 text-xs leading-5 text-slate-400"><li>• Project name — Placeholder</li><li>• Builder contact — Placeholder</li><li>• Ecosystem target — Placeholder</li><li>• Intake status — Placeholder</li></ul></div></div></div>;
}
