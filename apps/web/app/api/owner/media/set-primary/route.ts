import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const body = await req.json();
  if (!isOwnerRequest(String(body.password ?? ""))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const mediaId = String(body.media_id ?? "");
  if (!mediaId) return NextResponse.json({ error: "missing media_id" }, { status: 400 });

  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "db unavailable" }, { status: 500 });
  const media = (await pool.query("select id, product_slug from product_media where id=$1", [mediaId])).rows[0];
  if (!media) return NextResponse.json({ error: "not found" }, { status: 404 });

  await pool.query("update product_media set is_primary=false, updated_at=now() where product_slug=$1", [media.product_slug]);
  await pool.query("update product_media set is_primary=true, updated_at=now() where id=$1", [mediaId]);

  return NextResponse.json({ ok: true });
}
