import { NextResponse } from "next/server";
import { gameFactoryDb } from "@/lib/game-factory";

export async function GET(_req: Request, { params }: { params: Promise<{ id: string }> }) {
  try {
    const { id } = await params;
    const project = await gameFactoryDb.getProject(id);
    if (!project) return NextResponse.json({ ok:false, error:"not_found" },{status:404});
    return NextResponse.json({ ok:true, project, brief: await gameFactoryDb.getBrief(id), assets: await gameFactoryDb.getAssets(id), files: await gameFactoryDb.getFiles(id) });
  } catch { return NextResponse.json({ ok:false, error:"failed_to_fetch_project" },{status:500}); }
}
