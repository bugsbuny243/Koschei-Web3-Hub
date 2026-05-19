import { NextResponse } from "next/server";
import { gameFactoryDb } from "@/lib/game-factory";

export async function GET(_req: Request, { params }: { params: { id: string } }) {
  try {
    const project = await gameFactoryDb.getProject(params.id);
    if (!project) return NextResponse.json({ ok:false, error:"not_found" },{status:404});
    return NextResponse.json({ ok:true, project, brief: await gameFactoryDb.getBrief(params.id), assets: await gameFactoryDb.getAssets(params.id), files: await gameFactoryDb.getFiles(params.id) });
  } catch { return NextResponse.json({ ok:false, error:"failed_to_fetch_project" },{status:500}); }
}
