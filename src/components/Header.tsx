import Link from "next/link";
import { Button } from "@/components/Button";

export function Header() {
  return (
    <header className="border-b border-slate-200/80 bg-white/90 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center justify-between px-5 py-4 lg:px-8">
        <Link href="/" className="flex items-center gap-2 text-xl font-black tracking-tight text-slate-950">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-slate-950 text-sm text-cyan-400">TP</span>
          Teklif<span className="text-cyan-600">Pilot</span>
        </Link>
        <nav className="flex items-center gap-2 sm:gap-4">
          <Link href="/dashboard" className="hidden text-sm font-semibold text-slate-600 hover:text-slate-950 sm:block">Panel</Link>
          <Button href="/quote/new" className="px-4 py-2.5">Teklif Oluştur <span className="ml-1">→</span></Button>
        </nav>
      </div>
    </header>
  );
}
