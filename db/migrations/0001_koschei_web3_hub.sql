CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$ BEGIN CREATE TYPE order_provider AS ENUM ('shopier', 'manual'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE order_status AS ENUM ('pending', 'paid', 'manual_verified', 'failed', 'cancelled', 'refunded'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE entitlement_status AS ENUM ('active', 'exhausted', 'cancelled'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE generated_output_type AS ENUM ('game_asset', 'risk_report', 'launch_copy', 'docs_bundle'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), slug text NOT NULL UNIQUE, name text NOT NULL,
  price_try_cents integer NOT NULL CHECK (price_try_cents > 0), output_quota integer NOT NULL CHECK (output_quota > 0),
  shopier_url text NOT NULL, is_active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS customers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text NOT NULL UNIQUE, full_name text NOT NULL,
  company text, country text, source text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid NOT NULL REFERENCES customers(id), product_id uuid NOT NULL REFERENCES products(id),
  provider order_provider NOT NULL, provider_order_id text, amount_try_cents integer NOT NULL CHECK (amount_try_cents > 0),
  status order_status NOT NULL DEFAULT 'pending', raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_order_id)
);
CREATE TABLE IF NOT EXISTS entitlements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid NOT NULL REFERENCES customers(id), order_id uuid NOT NULL UNIQUE REFERENCES orders(id),
  product_id uuid NOT NULL REFERENCES products(id), outputs_total integer NOT NULL CHECK (outputs_total > 0),
  outputs_remaining integer NOT NULL CHECK (outputs_remaining >= 0 AND outputs_remaining <= outputs_total), status entitlement_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid NOT NULL REFERENCES customers(id), name text NOT NULL,
  ecosystem text NOT NULL, project_type text NOT NULL, description text, status text NOT NULL DEFAULT 'draft',
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS generated_outputs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid NOT NULL REFERENCES customers(id), entitlement_id uuid NOT NULL REFERENCES entitlements(id),
  project_id uuid REFERENCES projects(id), output_type generated_output_type NOT NULL, metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  content_json jsonb, content_text text, used_ai boolean NOT NULL DEFAULT false, used_fallback boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS risk_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid REFERENCES projects(id), generated_output_id uuid REFERENCES generated_outputs(id),
  score integer CHECK (score BETWEEN 0 AND 100), severity text, findings jsonb NOT NULL DEFAULT '[]'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS chain_health_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), chain text NOT NULL, network text, provider text, is_healthy boolean NOT NULL,
  latency_ms integer, details jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS ai_generation_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid REFERENCES customers(id), project_id uuid REFERENCES projects(id), generated_output_id uuid REFERENCES generated_outputs(id),
  mode text NOT NULL, provider text, model text, used_ai boolean NOT NULL DEFAULT false, used_fallback boolean NOT NULL DEFAULT false,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS intake_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), customer_id uuid REFERENCES customers(id), email text NOT NULL, full_name text, company text,
  ecosystem text, project_type text, description text, status text NOT NULL DEFAULT 'new', metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS admin_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), admin_email text NOT NULL, action text NOT NULL, entity_type text NOT NULL, entity_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS orders_customer_id_idx ON orders(customer_id);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders(status);
CREATE INDEX IF NOT EXISTS entitlements_customer_id_idx ON entitlements(customer_id);
CREATE INDEX IF NOT EXISTS generated_outputs_entitlement_id_idx ON generated_outputs(entitlement_id);

INSERT INTO products (slug, name, price_try_cents, output_quota, shopier_url, is_active) VALUES
  ('starter', 'Starter Pack', 89900, 1, 'https://www.shopier.com/TradeVisual/47465449', true),
  ('builder', 'Builder Pack', 229900, 3, 'https://www.shopier.com/TradeVisual/47465484', true),
  ('studio', 'Studio Pack', 499900, 10, 'https://www.shopier.com/TradeVisual/47465499', true)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, price_try_cents = EXCLUDED.price_try_cents, output_quota = EXCLUDED.output_quota, shopier_url = EXCLUDED.shopier_url, is_active = EXCLUDED.is_active, updated_at = now();
