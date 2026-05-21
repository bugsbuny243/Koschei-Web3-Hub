"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

export default function OwnerLoginPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    const res = await fetch("/api/owner/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ password }) });
    if (!res.ok) {
      const out = await res.json().catch(() => ({}));
      setError(out?.error ?? "Unauthorized");
      setLoading(false);
      return;
    }
    router.push("/owner/command-center");
    router.refresh();
  }

  return <main className="page-stack"><h1>Owner Login</h1><form className="card" onSubmit={onSubmit}><label>Admin Password<input type="password" value={password} onChange={(e)=>setPassword(e.target.value)} required /></label><button className="btn btn-primary" type="submit" disabled={loading}>{loading ? "Giriş yapılıyor..." : "Giriş Yap"}</button>{error && <p>{error}</p>}</form></main>;
}
