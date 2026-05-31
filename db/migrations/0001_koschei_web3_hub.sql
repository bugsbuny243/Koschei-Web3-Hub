BEGIN;

-- Extend existing commerce tables without removing or rewriting legacy data.
ALTER TABLE plans
  ADD COLUMN IF NOT EXISTS pack_type TEXT,
  ADD COLUMN IF NOT EXISTS output_quota INTEGER DEFAULT 1,
  ADD COLUMN IF NOT EXISTS shopier_url TEXT,
  ADD COLUMN IF NOT EXISTS description TEXT,
  ADD COLUMN IF NOT EXISTS image_url TEXT;

ALTER TABLE payment_requests
  ADD COLUMN IF NOT EXISTS full_name TEXT,
  ADD COLUMN IF NOT EXISTS product_id TEXT,
  ADD COLUMN IF NOT EXISTS amount_try INTEGER,
  ADD COLUMN IF NOT EXISTS currency TEXT DEFAULT 'TRY',
  ADD COLUMN IF NOT EXISTS raw_payload JSONB DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Reuse the existing application profile table for public member authentication.
-- Public signup is delegated to Neon Auth and only creates a standard user profile.
-- Admin access remains owner-only through ADMIN_EMAIL / ADMIN_PASSWORD.
CREATE TABLE IF NOT EXISTS app_user_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_subject TEXT UNIQUE NOT NULL,
  email TEXT UNIQUE NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  plan_id TEXT NOT NULL DEFAULT 'free',
  credits INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS app_user_profiles_lower_email_idx ON app_user_profiles (lower(email));

-- Web3 Hub entitlement and generated-output persistence layer.
CREATE TABLE IF NOT EXISTS entitlements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  plan_id TEXT REFERENCES plans(id),
  payment_request_id UUID REFERENCES payment_requests(id),
  outputs_total INTEGER NOT NULL,
  outputs_remaining INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  starts_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS web3_projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  title TEXT NOT NULL,
  ecosystem TEXT,
  project_type TEXT,
  description TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  metadata JSONB DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS web3_outputs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  project_id UUID REFERENCES web3_projects(id) ON DELETE CASCADE,
  entitlement_id UUID REFERENCES entitlements(id),
  output_type TEXT NOT NULL,
  title TEXT,
  ecosystem TEXT,
  content_json JSONB DEFAULT '{}'::jsonb,
  content_text TEXT,
  used_ai BOOLEAN DEFAULT false,
  used_fallback BOOLEAN DEFAULT false,
  ai_model TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS risk_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  project_id UUID REFERENCES web3_projects(id) ON DELETE CASCADE,
  score INTEGER NOT NULL,
  risk_level TEXT NOT NULL,
  checklist JSONB NOT NULL DEFAULT '{}'::jsonb,
  recommended_fixes JSONB NOT NULL DEFAULT '[]'::jsonb,
  disclaimer TEXT NOT NULL DEFAULT 'This is an informational risk checklist, not legal, financial, investment, or security advice.',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chain_health_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  chain TEXT NOT NULL,
  network TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT 'alchemy',
  ok BOOLEAN NOT NULL DEFAULT false,
  result TEXT,
  error TEXT,
  checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_generation_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT,
  mode TEXT NOT NULL,
  provider TEXT,
  model TEXT,
  prompt JSONB DEFAULT '{}'::jsonb,
  output_text TEXT,
  used_fallback BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS entitlements_payment_request_id_idx ON entitlements (payment_request_id);
CREATE INDEX IF NOT EXISTS entitlements_email_idx ON entitlements (email);
CREATE INDEX IF NOT EXISTS entitlements_status_idx ON entitlements (status);
CREATE INDEX IF NOT EXISTS web3_projects_email_idx ON web3_projects (email);
CREATE INDEX IF NOT EXISTS web3_outputs_email_idx ON web3_outputs (email);
CREATE INDEX IF NOT EXISTS web3_outputs_project_id_idx ON web3_outputs (project_id);
CREATE INDEX IF NOT EXISTS risk_reports_project_id_idx ON risk_reports (project_id);
CREATE INDEX IF NOT EXISTS chain_health_logs_chain_network_idx ON chain_health_logs (chain, network);
CREATE INDEX IF NOT EXISTS ai_generation_logs_email_idx ON ai_generation_logs (email);

INSERT INTO plans (id, name, price_try, monthly_credits, pack_type, output_quota, shopier_url)
VALUES
  (
    'starter',
    'Koschei Starter Pack – Web3 Project Output',
    899,
    1,
    'starter',
    1,
    'https://www.shopier.com/TradeVisual/47465449'
  ),
  (
    'builder',
    'Koschei Builder Pack – Metadata + Risk + Launch',
    2299,
    3,
    'builder',
    3,
    'https://www.shopier.com/TradeVisual/47465484'
  ),
  (
    'studio',
    'Koschei Studio Pack – Web3 Builder Studio',
    4999,
    10,
    'studio',
    10,
    'https://www.shopier.com/TradeVisual/47465499'
  )
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  price_try = EXCLUDED.price_try,
  monthly_credits = EXCLUDED.monthly_credits,
  pack_type = EXCLUDED.pack_type,
  output_quota = EXCLUDED.output_quota,
  shopier_url = EXCLUDED.shopier_url;

COMMIT;
