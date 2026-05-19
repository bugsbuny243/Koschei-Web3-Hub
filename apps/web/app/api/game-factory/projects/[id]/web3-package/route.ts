import { NextResponse } from "next/server";
import { buildNftMetadata, buildWeb3BridgeConfig, gameFactoryDb } from "@/lib/game-factory";
import { web3Db } from "@/lib/web3-db";

export async function POST(_req: Request, { params }: { params: { id: string } }) {
  const project = await gameFactoryDb.getProject(params.id);
  if (!project) return NextResponse.json({ error: "not found" }, { status: 404 });
  const items = project.extracted_items ?? [];
  const metadata = buildNftMetadata(items);
  const bridge_config = buildWeb3BridgeConfig(project.id);
  const { rows } = await web3Db.query<{ id: string }>(`insert into game_factory_web3_packages (project_id, chain_slug, nft_metadata, bridge_config, export_bundle) values ($1,$2,$3::jsonb,$4::jsonb,$5::jsonb) returning id::text`, [project.id, "arbitrum-sepolia", JSON.stringify(metadata), JSON.stringify(bridge_config), JSON.stringify({ files: ["game.js", "metadata.json", "bridge-config.json"] })]);
  return NextResponse.json({ package_id: rows[0].id, metadata, bridge_config });
}
