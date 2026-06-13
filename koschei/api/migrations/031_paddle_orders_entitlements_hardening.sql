CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text UNIQUE NOT NULL,
  name text NOT NULL,
  pack_type text NOT NULL,
  description text NOT NULL DEFAULT '',
  price numeric(20,2) NOT NULL DEFAULT 0,
  output_quota integer NOT NULL DEFAULT 0,
  shopier_url text NOT NULL DEFAULT '',
  paddle_price_id text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE products ADD COLUMN IF NOT EXISTS slug text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS name text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS pack_type text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS price numeric(20,2) NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS output_quota integer NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS shopier_url text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS paddle_price_id text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true;
ALTER TABLE products ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE products ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE UNIQUE INDEX IF NOT EXISTS products_slug_unique_idx ON products (slug);
CREATE INDEX IF NOT EXISTS products_active_pack_type_idx ON products (is_active, pack_type);

INSERT INTO products (slug, name, pack_type, description, price, output_quota, is_active)
VALUES
  ('starter', 'Starter', 'starter', 'Koschei Starter package', 899, 25, true),
  ('professional', 'Professional', 'professional', 'Koschei Professional package', 2299, 100, true),
  ('enterprise', 'Enterprise', 'enterprise', 'Koschei Enterprise package', 4999, 300, true)
ON CONFLICT (slug) DO UPDATE SET
  name = EXCLUDED.name,
  pack_type = EXCLUDED.pack_type,
  description = COALESCE(NULLIF(products.description, ''), EXCLUDED.description),
  output_quota = GREATEST(COALESCE(products.output_quota, 0), EXCLUDED.output_quota),
  is_active = true,
  updated_at = now();

CREATE TABLE IF NOT EXISTS orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id text,
  product_id text,
  provider text NOT NULL DEFAULT 'paddle',
  provider_order_id text,
  provider_payment_id text,
  amount numeric(20,2) NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT 'USD',
  status text NOT NULL DEFAULT 'pending',
  raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  purchased_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS customer_id text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS product_id text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'paddle';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider_order_id text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS provider_payment_id text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS amount numeric(20,2) NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'USD';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS purchased_at timestamptz;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE orders ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS orders_provider_order_idx ON orders (provider, provider_order_id);
CREATE INDEX IF NOT EXISTS orders_provider_payment_idx ON orders (provider, provider_payment_id);
CREATE INDEX IF NOT EXISTS orders_customer_created_idx ON orders (lower(customer_id), created_at DESC);

ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS starts_at timestamptz;
ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS order_id uuid;
ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS product_id text;
ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS payment_provider text;
ALTER TABLE entitlements ADD COLUMN IF NOT EXISTS external_payment_id text;
CREATE INDEX IF NOT EXISTS entitlements_email_status_expires_idx ON entitlements (lower(email), status, expires_at);
CREATE INDEX IF NOT EXISTS entitlements_order_id_idx ON entitlements (order_id);
CREATE UNIQUE INDEX IF NOT EXISTS entitlements_one_active_paid_per_email_idx
  ON entitlements (lower(email))
  WHERE status = 'active' AND COALESCE(plan_id, '') <> '' AND COALESCE(plan_id, '') <> 'free';
