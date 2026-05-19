export const dynamic = 'force-dynamic';
import { notFound } from "next/navigation";
import { gameFactoryDb } from "@/lib/game-factory";
import { GameFactoryGenerateButton } from "@/components/game-factory-generate-button";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const p = await gameFactoryDb.getProject(id);
  if (!p) return notFound();
  const files = await gameFactoryDb.getFiles(p.id);
  const html = (files.find((f) => f.file_type === "html") as { content?: string } | undefined)?.content;
  return <main className="mx-auto max-w-5xl space-y-4 p-6"><h1 className="text-3xl font-bold">Live Preview</h1>{html ? <iframe title="preview" className="h-[420px] w-full rounded border" srcDoc={html} /> : <div className="rounded border p-4 space-y-3"><p>No generated game preview yet.</p><GameFactoryGenerateButton id={p.id} kind="preview" /></div>}</main>;
}
