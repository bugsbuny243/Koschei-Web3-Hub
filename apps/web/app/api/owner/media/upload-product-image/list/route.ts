import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const password = searchParams.get("password");
  const productSlug = searchParams.get("product_slug");
  if (!(await isOwnerAuthenticated(password))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  if (!productSlug) return NextResponse.json({ media: [] });
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ media: [] });
  const media = (await pool.query("select * from product_media where product_slug=$1 order by is_primary desc, created_at desc", [productSlug])).rows;
  return NextResponse.json({ media });
}
