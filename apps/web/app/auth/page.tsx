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
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#0a0a0a] px-4 py-10">
      <div className="auth-animated-bg pointer-events-none absolute inset-0" aria-hidden="true" />
      <section className="relative z-10 w-full max-w-md rounded-2xl border border-[#1e1e1e] bg-[#101010]/90 p-8 shadow-[0_0_60px_rgba(0,0,0,0.45)] backdrop-blur-sm">
        <div className="mb-7 text-center">
          <p className="text-3xl font-extrabold tracking-[0.2em] text-[#00ff87]">KOSCHEI</p>
          <p className="mt-2 text-sm font-medium uppercase tracking-[0.22em] text-[#00ff87]/80">The Immortal AI</p>
        </div>

        <h1 className="mb-5 text-center text-2xl font-semibold text-zinc-100">
          {mode === "login" ? "Sign in" : "Create account"}
        </h1>

        <form onSubmit={onSubmit} className="grid grid-cols-1 gap-4">
          <label className="grid gap-2 text-sm font-medium text-zinc-300">
            Email
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-lg border border-[#262626] bg-[#121212] px-3 py-2.5 text-zinc-100 outline-none transition focus:border-[#00ff87] focus:ring-2 focus:ring-[#00ff87]/20"
            />
          </label>

          <label className="grid gap-2 text-sm font-medium text-zinc-300">
            Password
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-lg border border-[#262626] bg-[#121212] px-3 py-2.5 text-zinc-100 outline-none transition focus:border-[#00ff87] focus:ring-2 focus:ring-[#00ff87]/20"
            />
          </label>

          {error ? <p className="text-sm font-medium text-red-400">{error}</p> : null}

          <button
            className="mt-1 w-full rounded-lg bg-[#00ff87] px-4 py-2.5 font-semibold text-[#0a0a0a] transition hover:bg-[#1dff98] disabled:cursor-not-allowed disabled:opacity-70"
            disabled={loading}
          >
            {loading ? "Please wait..." : mode === "login" ? "Login" : "Register"}
          </button>
        </form>

        <button
          className="mt-4 w-full rounded-lg border border-[#2b2b2b] bg-[#151515] px-4 py-2.5 font-medium text-zinc-200 transition hover:border-[#00ff87]/70 hover:text-[#00ff87]"
          onClick={() => setMode(mode === "login" ? "register" : "login")}
        >
          Switch to {mode === "login" ? "Register" : "Login"}
        </button>
      </section>
    </div>
  );
}
