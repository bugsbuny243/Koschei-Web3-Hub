import Link from "next/link";
import { SupportCta } from "@/components/support-cta";
import { gameFactoryPositioning, gameFactorySafetyCopy } from "@/lib/game-factory";

export default function Page(){return <main className="mx-auto max-w-6xl space-y-6 p-6"><h1 className="text-4xl font-bold">Koschei Web Game Factory</h1><p>{gameFactoryPositioning}</p><div className="flex gap-3"><Link href="/game-factory/new" className="rounded bg-black px-4 py-2 text-white">Create Game</Link><Link href="/game-factory/projects" className="rounded border px-4 py-2">Projects</Link></div><p className="rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p><SupportCta compact /></main>}
