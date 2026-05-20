import fs from "node:fs/promises";
import path from "node:path";

async function getNotes() {
  const filePath = path.join(process.cwd(), "data/internal/supplier-notes.json");
  return JSON.parse(await fs.readFile(filePath, "utf8")) as Array<Record<string, unknown>>;
}

export default async function SupplierNotesPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }
  const notes = await getNotes();
  return <div className="page-stack"><h1>Supplier Notes</h1><p>Internal supplier data. Do not publish prices, bank details or private negotiation notes.</p><pre>{JSON.stringify(notes, null, 2)}</pre></div>;
}
