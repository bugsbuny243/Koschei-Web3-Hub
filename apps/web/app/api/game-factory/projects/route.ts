import { NextResponse } from "next/server";
import { gameFactoryDb } from "@/lib/game-factory";

export async function GET() {
  try { return NextResponse.json({ ok: true, projects: await gameFactoryDb.listProjects() }); }
  catch { return NextResponse.json({ ok:false, error:"failed_to_list_projects" },{status:500}); }
}

export async function POST(req: Request) {
  try {
    const b = await req.json();
    if (!b?.prompt || typeof b.prompt !== "string") return NextResponse.json({ ok:false, error:"prompt_required" },{status:400});
    const project = await gameFactoryDb.createProject({ title:b.title, prompt:b.prompt, genre:b.genre, visual_style:b.style, target_chain:b.target_chain || "arbitrum-sepolia" });
    return NextResponse.json({ ok:true, project }, { status: 201 });
  } catch { return NextResponse.json({ ok:false, error:"failed_to_create_project" },{status:500}); }
}
