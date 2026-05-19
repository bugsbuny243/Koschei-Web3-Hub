import { NextResponse } from "next/server";
import { buildAssets, buildGameBrief, buildPreviewHtml, gameFactoryDb } from "@/lib/game-factory";
import { web3Db } from "@/lib/web3-db";

export async function POST(_req: Request, { params }: { params: { id: string } }) {
  try {
    const p = await gameFactoryDb.getProject(params.id);
    if (!p) return NextResponse.json({ ok:false, error:"not_found" },{status:404});
    const brief = buildGameBrief({ title:p.title, prompt:p.prompt, genre:p.genre, style:p.visual_style });
    const preview = buildPreviewHtml(brief);
    const assets = buildAssets(brief);
    await web3Db.query("insert into game_factory_briefs (project_id, brief) values ($1,$2::jsonb)",[p.id, JSON.stringify(brief)]);
    for (const a of assets) await web3Db.query("insert into game_factory_assets (project_id, asset_type, name, description, rarity, metadata) values ($1,$2,$3,$4,$5,$6::jsonb)",[p.id,a.asset_type,a.name,a.description,a.rarity,JSON.stringify(a.metadata)]);
    await web3Db.query("insert into game_factory_generated_files (project_id, file_path, file_type, content, metadata) values ($1,$2,$3,$4,$5::jsonb)",[p.id,"preview/index.html","html",preview,JSON.stringify({generated:true})]);
    await web3Db.query("update game_factory_projects set status='generated', updated_at=now() where id=$1",[p.id]);
    return NextResponse.json({ ok:true, brief, assets });
  } catch { return NextResponse.json({ ok:false, error:"failed_to_generate" },{status:500}); }
}
