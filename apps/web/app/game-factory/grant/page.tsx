import { gameFactoryPositioning, gameFactorySafetyCopy } from "@/lib/game-factory";

export default function GameFactoryGrantPage() {
  return <main className="mx-auto max-w-4xl space-y-4 p-6"><h1 className="text-3xl font-bold">Web Game Factory Grant Narrative</h1><p>{gameFactoryPositioning}</p><p>Core MVP: prompt → brief → Phaser template → preview → item/reward extraction → NFT metadata JSON → Arbitrum Sepolia bridge config → Web3-ready package export.</p><p className="rounded bg-amber-100 p-3 text-sm">{gameFactorySafetyCopy}</p></main>;
}
