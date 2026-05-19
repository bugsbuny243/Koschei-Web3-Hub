export const dynamic = 'force-dynamic';
import { notFound } from "next/navigation";
import { gameFactoryDb, gameFactorySafetyCopy } from "@/lib/game-factory";

export default async function Page({params}:{params:{id:string}}){const p=await gameFactoryDb.getProject(params.id);if(!p)return notFound();const pkg=await gameFactoryDb.getWeb3Package(p.id);return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Web3-ready Package</h1><p className="rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p><pre className="rounded bg-gray-100 p-3 text-xs overflow-auto">{JSON.stringify(pkg ?? {message:"Generate package via API POST /api/game-factory/projects/[id]/web3-package"},null,2)}</pre></main>;}
