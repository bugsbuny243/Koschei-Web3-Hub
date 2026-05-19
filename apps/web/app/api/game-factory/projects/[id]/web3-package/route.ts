import { NextResponse } from "next/server";
import { buildWeb3Package, gameFactoryDb } from "@/lib/game-factory";
import { web3Db } from "@/lib/web3-db";

type RouteContext = {
  params: Promise<{ id: string }>;
};

function jsonError(status: number, error: string, message: string) {
  return NextResponse.json({ ok: false, error, message }, { status });
}

export async function GET(_req: Request, context: RouteContext) {
  try {
    const { id } = await context.params;
    const project = await gameFactoryDb.getProject(id);
    if (!project) return jsonError(404, "not_found", "Project not found");

    const existingPackage = await gameFactoryDb.getWeb3Package(project.id);
    return NextResponse.json({ ok: true, package: existingPackage });
  } catch {
    return jsonError(500, "failed_to_load_package", "Failed to load Web3 package");
  }
}

export async function POST(_req: Request, context: RouteContext) {
  try {
    const { id } = await context.params;
    const project = await gameFactoryDb.getProject(id);
    if (!project) {
      return NextResponse.json({ ok: false, error: "project_not_found" }, { status: 404 });
    }

    const brief = await gameFactoryDb.getBrief(project.id);
    if (!brief) return jsonError(400, "generate_first", "Generate project content first");

    const assets = await gameFactoryDb.getAssets(project.id);
    const pkg = buildWeb3Package(project, brief, assets);

    await web3Db.query(
      `insert into game_factory_web3_packages (project_id, target_chain, manifest, item_schema, nft_metadata, reward_config, adapter_config)
       values ($1,$2,$3::jsonb,$4::jsonb,$5::jsonb,$6::jsonb,$7::jsonb)
       on conflict (project_id)
       do update set
         target_chain = excluded.target_chain,
         manifest = excluded.manifest,
         item_schema = excluded.item_schema,
         nft_metadata = excluded.nft_metadata,
         reward_config = excluded.reward_config,
         adapter_config = excluded.adapter_config,
         updated_at = now()`,
      [
        project.id,
        project.target_chain,
        JSON.stringify(pkg.manifest),
        JSON.stringify(pkg.item_schema),
        JSON.stringify(pkg.nft_metadata),
        JSON.stringify(pkg.reward_config),
        JSON.stringify(pkg.adapter_config)
      ]
    );

    return NextResponse.json({ ok: true, package: pkg });
  } catch {
    return jsonError(500, "failed_to_build_package", "Failed to build Web3 package");
  }
}
