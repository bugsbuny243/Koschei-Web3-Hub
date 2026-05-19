export const dynamic = 'force-dynamic';
import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";

export default async function Page({ params }: { params: { id: string } }) {
  const p = await gameFactoryDb.getProject(params.id);
  if (!p) return notFound();
  const files = await gameFactoryDb.getFiles(p.id);
  const html = (files.find((f) => f.file_type === "html") as { content?: string } | undefined)?.content;
  return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Live Preview</h1>{html ? <iframe title="preview" className="h-[420px] w-full rounded border" srcDoc={html} /> : <p className="rounded border p-4">Generate first via POST /api/game-factory/projects/[id]/generate</p>}</main>;
}
