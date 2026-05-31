import Link from "next/link";

const links = [["Hub", "/hub"], ["Builder", "/builder"], ["Metadata", "/metadata"], ["Risk", "/risk"], ["Chains", "/chains"], ["Ecosystem", "/ecosystem"], ["Docs", "/docs"], ["Admin", "/admin"]];

export function SiteHeader() {
  return <header className="sticky top-0 z-50 border-b border-white/10 bg-[#050816]/90 backdrop-blur-xl"><div className="mx-auto flex max-w-7xl items-center justify-between gap-5 px-5 py-4 lg:px-8"><Link href="/" className="flex items-center gap-3 font-black tracking-tight text-white"><span className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-violet-600 text-sm shadow-lg shadow-blue-500/20">K</span><span>Koschei <span className="text-blue-400">Web3 Hub</span></span></Link><nav className="hidden gap-5 lg:flex">{links.map(([label, href]) => <Link key={href} href={href} className="text-sm font-semibold text-slate-400 hover:text-white">{label}</Link>)}</nav><Link href="/hub" className="rounded-lg border border-blue-400/40 bg-blue-500/10 px-3 py-2 text-xs font-bold text-blue-200 hover:bg-blue-500/20">Open Hub</Link></div><nav className="flex gap-4 overflow-x-auto px-5 pb-3 lg:hidden">{links.map(([label, href]) => <Link key={href} href={href} className="shrink-0 text-xs font-semibold text-slate-400 hover:text-white">{label}</Link>)}</nav></header>;
}
