-- TradePi product_media Cloudinary compatibility migration
-- Safe/idempotent migration

alter table if exists product_media
  add column if not exists product_slug text,
  add column if not exists provider text default 'cloudinary',
  add column if not exists public_id text,
  add column if not exists secure_url text,
  add column if not exists original_filename text,
  add column if not exists status text default 'ready',
  add column if not exists uploaded_by text,
  add column if not exists updated_at timestamptz default now();

create index if not exists idx_product_media_product_slug on product_media(product_slug);
create index if not exists idx_product_media_product_slug_primary on product_media(product_slug, is_primary);
