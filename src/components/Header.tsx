import Link from "next/link";

const links = [
  ["Hub", "/hub"], ["Builder", "/builder"], ["Metadata", "/metadata"], ["Risk", "/risk"],
  ["Chains", "/chains"], ["Ecosystem", "/ecosystem"], ["Docs", "/docs"],
];

export function Header() {
  return <header className="border-b border-white/10 bg-[#050816]/95 text-white backdrop-blur">
    <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-5 py-4 lg:px-8">
      <Link href="/" className="flex items-center gap-3 text-lg font-black tracking-tight"><span className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400 to-violet-500 text-xs text-slate-950">K</span><span>Koschei <span className="text-cyan-400">Web3 Hub</span></span></Link>
      <nav className="order-3 flex w-full gap-4 overflow-x-auto pb-1 text-xs font-bold text-slate-300 lg:order-2 lg:w-auto lg:pb-0">{links.map(([label, href]) => <Link key={href} href={href} className="whitespace-nowrap hover:text-cyan-300">{label}</Link>)}</nav>
      <div className="order-2 flex items-center gap-2 lg:order-3"><Link href="/login" className="px-2 py-2 text-xs font-bold text-slate-300 hover:text-white">Sign in</Link><Link href="/signup" className="rounded-lg border border-cyan-400/50 bg-cyan-400/10 px-3 py-2 text-xs font-bold text-cyan-200 hover:bg-cyan-400/20">Get started</Link></div>
    </div>
  </header>;
}
