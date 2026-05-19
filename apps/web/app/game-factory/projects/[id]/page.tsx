export const dynamic = 'force-dynamic';
import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";
import { GameFactoryActions } from "@/components/game-factory-actions";

export default async function Page({params}:{params:{id:string}}){const p=await gameFactoryDb.getProject(params.id);if(!p)return notFound();const brief=await gameFactoryDb.getBrief(p.id);const files=await gameFactoryDb.getFiles(p.id);const assets=await gameFactoryDb.getAssets(p.id);return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">{p.title||"Untitled project"}</h1><p>{p.prompt}</p><GameFactoryActions id={p.id} /><pre className="rounded bg-gray-100 p-3 text-xs">{JSON.stringify({brief,files,assets},null,2)}</pre></main>;}
