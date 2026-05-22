"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

export default function AuthPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [mode, setMode] = useState<"login" | "register">("login");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/auth", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: mode, email, password }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error ?? "Failed");
      localStorage.setItem("koschei_token", data.token);
      localStorage.setItem("koschei_user", JSON.stringify(data.user));
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="container page-stack">
      <section className="card" style={{ maxWidth: 480, margin: "0 auto" }}>
        <h1>{mode === "login" ? "Sign in" : "Create account"}</h1>
        <form onSubmit={onSubmit} className="form-grid" style={{ gridTemplateColumns: "1fr" }}>
          <label>Email<input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} /></label>
          <label>Password<input type="password" required minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} /></label>
          {error ? <p style={{ color: "#b91c1c" }}>{error}</p> : null}
          <button className="btn btn-primary" disabled={loading}>{loading ? "Please wait..." : mode === "login" ? "Login" : "Register"}</button>
        </form>
        <button className="btn btn-secondary" style={{ marginTop: 12 }} onClick={() => setMode(mode === "login" ? "register" : "login")}>Switch to {mode === "login" ? "Register" : "Login"}</button>
      </section>
    </div>
  );
}
