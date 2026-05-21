import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";
import { uploadProductImage } from "@/lib/media/cloudinary";

export async function POST(req: Request) {
  const form = await req.formData();
  const password = String(form.get("password") ?? "");
  if (!(await isOwnerAuthenticated(password))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  if (process.env.MEDIA_PROVIDER !== "cloudinary") return NextResponse.json({ error: "MEDIA_PROVIDER must be cloudinary" }, { status: 400 });

  const productSlug = String(form.get("product_slug") ?? "").trim();
  const altText = String(form.get("alt_text") ?? "").trim() || null;
  const isPrimary = String(form.get("is_primary") ?? "false") === "true";
  const file = form.get("file");
  if (!productSlug || !(file instanceof File)) return NextResponse.json({ error: "missing product_slug or file" }, { status: 400 });

  const uploaded = await uploadProductImage(file, productSlug);
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "db unavailable" }, { status: 500 });

  if (isPrimary) await pool.query("update product_media set is_primary=false, updated_at=now() where product_slug=$1", [productSlug]);

  const row = (await pool.query(
    `insert into product_media (product_slug,media_type,title,file_path,alt_text,is_primary,provider,public_id,secure_url,original_filename,status,uploaded_by,updated_at)
     values ($1,'image',$2,$3,$4,$5,'cloudinary',$6,$7,$8,'ready','owner',now()) returning *`,
    [productSlug, file.name, uploaded.secure_url, altText, isPrimary, uploaded.public_id, uploaded.secure_url, uploaded.original_filename ?? file.name],
  )).rows[0];

  return NextResponse.json({ ok: true, media: row });
}
