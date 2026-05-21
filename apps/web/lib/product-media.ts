import "server-only";
import { getDbPool } from "@/lib/db";

export async function getPrimaryMediaMap(slugs: string[]) {
  const pool = getDbPool();
  if (!pool || slugs.length === 0) return new Map<string, string>();
  const rows = (await pool.query(
    `select distinct on (product_slug) product_slug, coalesce(secure_url, file_path) as url
     from product_media where product_slug = any($1::text[])
     order by product_slug, is_primary desc, created_at desc`,
    [slugs],
  )).rows as Array<{ product_slug: string; url: string }>;
  return new Map(rows.map((r) => [r.product_slug, r.url]));
}

export async function getPrimaryMedia(slug: string) {
  const m = await getPrimaryMediaMap([slug]);
  return m.get(slug) ?? null;
}
