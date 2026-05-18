import { NextResponse } from "next/server";
import { gameBridgeDb } from "@/lib/game-bridge";

export async function GET() {
  const rows = await gameBridgeDb.listItems();
  return NextResponse.json({ items: rows });
}

export async function POST(req: Request) {
  const body = await req.json();
  if (!body?.item_key || !body?.name) return NextResponse.json({ error: "item_key and name are required" }, { status: 400 });
  const created = await gameBridgeDb.createItem(body);
  return NextResponse.json({ item: created }, { status: 201 });
}
