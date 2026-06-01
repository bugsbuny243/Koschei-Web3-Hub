"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

type SessionState = "loading" | "public" | "member";

export function MemberHeaderActions() {
  const [session, setSession] = useState<SessionState>("loading");

  useEffect(() => {
    let active = true;
    fetch("/api/auth/me", { cache: "no-store" })
      .then((response) => response.ok ? response.json() as Promise<{ loggedIn?: boolean }> : null)
      .then((payload) => { if (active) setSession(payload?.loggedIn ? "member" : "public"); })
      .catch(() => { if (active) setSession("public"); });
    return () => { active = false; };
  }, []);

  if (session === "member") return <div className="flex shrink-0 items-center gap-2"><Link href="/dashboard" className="flex min-h-11 items-center px-2 text-xs font-bold text-slate-300 hover:text-white">Dashboard</Link><form action="/api/auth/logout" method="post"><button className="flex min-h-11 items-center px-2 text-xs font-bold text-slate-300 hover:text-white" type="submit">Sign out</button></form></div>;
  return <div className="flex shrink-0 items-center gap-2"><Link href="/login" className="flex min-h-11 items-center px-2 text-xs font-bold text-slate-300 hover:text-white">Sign in</Link><Link href="/signup" className="flex min-h-11 items-center rounded-lg border border-blue-400/40 bg-blue-500/10 px-3 py-2 text-xs font-bold text-blue-200 hover:bg-blue-500/20">Get started</Link></div>;
}
