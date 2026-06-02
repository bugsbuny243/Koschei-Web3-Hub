import Link from "next/link";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SiteHeader } from "@/components/SiteHeader";
import { ensureMemberProfile, getUserDashboard } from "@/lib/server/db";
import { getMemberSession } from "@/lib/server/auth-api";

const modules = [["Builder", "/builder", "Create portable game asset concepts."], ["Metadata", "/metadata", "Generate structured Web3 metadata."], ["Risk", "/risk", "Review transparent project signals."], ["Chains", "/chains", "Check supported testnet connectivity."]];

export default async function DashboardPage() {
  let session;
  try { session = await getMemberSession((await cookies()).toString()); } catch {
    return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-3xl px-5 py-16 lg:px-8"><p className="eyebrow">Member dashboard</p><h1 className="mt-4 text-4xl font-black text-white">Member sessions are unavailable</h1><p className="mt-4 text-sm leading-7 text-rose-200">The auth API is unavailable.</p></section></main>;
  }
  if (!session) redirect("/login");
  let dashboard;
  try {
    await ensureMemberProfile(session.sub, session.email);
    dashboard = await getUserDashboard(session.sub, session.email);
  } catch {
    return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-3xl px-5 py-16 lg:px-8"><p className="eyebrow">Member dashboard</p><h1 className="mt-4 text-4xl font-black text-white">Member account data is unavailable</h1><p className="mt-4 text-sm leading-7 text-rose-200">Confirm the member auth migration was applied.</p></section></main>;
  }
  if (!dashboard) redirect("/login");
  const hasPackage = Boolean(dashboard.plan_name);
  return <main className="web3-page"><SiteHeader /><section className="mx-auto max-w-7xl px-5 py-12 sm:py-16 lg:px-8"><div className="flex flex-col justify-between gap-5 sm:flex-row sm:items-end"><div><p className="eyebrow">Member dashboard</p><h1 className="mt-4 text-4xl font-black text-white">Your Web3 builder panel</h1><p className="mt-4 text-sm text-slate-400">Signed in as <strong className="text-slate-200">{dashboard.email}</strong></p></div><div className="flex flex-wrap gap-3"><form action="/api/auth/logout" method="post"><button className="web3-button-secondary" type="submit">Sign out</button></form></div></div><div className="mt-9 grid gap-5 md:grid-cols-3"><article className="glass-card p-6"><p className="eyebrow">Email</p><p className="web3-break-long mt-4 font-bold text-white">{dashboard.email}</p></article><article className="glass-card p-6"><p className="eyebrow">Plan / package</p><p className="mt-4 font-bold text-white">{dashboard.plan_name || "No active package"}</p><p className="mt-2 text-xs text-slate-400">Status: {dashboard.package_status || "not purchased"}</p></article><article className="glass-card p-6"><p className="eyebrow">Remaining outputs</p><p className="mt-4 text-4xl font-black text-white">{dashboard.outputs_remaining}</p><p className="mt-2 text-xs text-slate-400">Saved outputs: {dashboard.saved_outputs}</p></article></div>{!hasPackage ? <div className="mt-6 rounded-xl border border-amber-400/30 bg-amber-500/10 p-5 text-sm font-semibold text-amber-100">Buy a package to unlock saved outputs and usage rights.</div> : null}<div className="mt-10"><p className="eyebrow">Builder shortcuts</p><div className="mt-5 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">{modules.map(([title, href, description]) => <Link key={href} href={href} className="glass-card p-6 hover:border-blue-400/50"><h2 className="font-black text-white">{title} →</h2><p className="mt-3 text-sm leading-6 text-slate-400">{description}</p></Link>)}</div></div></section></main>;
}
