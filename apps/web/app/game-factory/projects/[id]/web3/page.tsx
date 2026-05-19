export const dynamic = 'force-dynamic';
import { notFound } from "next/navigation";
import { gameFactoryDb, gameFactorySafetyCopy } from "@/lib/game-factory";
import { GameFactoryGenerateButton } from "@/components/game-factory-generate-button";

const Block = ({ title, data }: { title: string; data: unknown }) => (
  <section className="space-y-2"><h2 className="font-semibold">{title}</h2><textarea readOnly className="min-h-40 w-full rounded border bg-gray-50 p-2 text-xs" value={JSON.stringify(data ?? {}, null, 2)} /></section>
);

export default async function Page({params}:{params:{id:string}}){const p=await gameFactoryDb.getProject(params.id);if(!p)return notFound();const pkg=await gameFactoryDb.getWeb3Package(p.id);return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Web3-ready Package</h1><p className="rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p>{!pkg ? <div className="rounded border p-4 space-y-3"><p>No package generated yet.</p><GameFactoryGenerateButton id={p.id} kind="web3" /></div> : <div className="space-y-4"><Block title="Manifest" data={pkg.manifest} /><Block title="Item Schema" data={pkg.item_schema} /><Block title="NFT Metadata" data={pkg.nft_metadata} /><Block title="Reward Config" data={pkg.reward_config} /><Block title="Adapter Config" data={pkg.adapter_config} /></div>}</main>;}
