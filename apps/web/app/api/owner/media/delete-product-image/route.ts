import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";
import { deleteProductImage } from "@/lib/media/cloudinary";

export async function POST(req: Request) {
  const body = await req.json();
  if (!isOwnerRequest(String(body.password ?? ""))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const mediaId = String(body.media_id ?? "");
  if (!mediaId) return NextResponse.json({ error: "missing media_id" }, { status: 400 });
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "db unavailable" }, { status: 500 });

  const row = (await pool.query("select id, public_id from product_media where id=$1", [mediaId])).rows[0];
  if (!row) return NextResponse.json({ error: "not found" }, { status: 404 });
  if (row.public_id) await deleteProductImage(row.public_id);
  await pool.query("delete from product_media where id=$1", [mediaId]);
  return NextResponse.json({ ok: true });
}
