CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  credits INTEGER NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profiles (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  display_name TEXT,
  bio TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS subscriptions (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES users(id) ON DELETE CASCADE, plan_name TEXT NOT NULL, status TEXT NOT NULL, period_start TIMESTAMPTZ, period_end TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS credit_ledger (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES users(id) ON DELETE CASCADE, delta INTEGER NOT NULL, reason TEXT NOT NULL, metadata JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS user_projects (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES users(id) ON DELETE CASCADE, name TEXT NOT NULL, project_type TEXT NOT NULL, content JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS ai_generations (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, project_id BIGINT REFERENCES user_projects(id) ON DELETE SET NULL, tool_type TEXT NOT NULL, model_used TEXT, prompt TEXT NOT NULL, output JSONB NOT NULL DEFAULT '{}'::jsonb, credits_spent INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());

CREATE TABLE IF NOT EXISTS owner_client_orders (id BIGSERIAL PRIMARY KEY, owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, source_platform TEXT NOT NULL DEFAULT 'direct', client_name TEXT, title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','requirements_received','in_production','ready_for_review','ready_for_delivery','delivered','revision_requested','completed','cancelled')), quoted_price NUMERIC(12,2), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_order_requirements (id BIGSERIAL PRIMARY KEY, order_id BIGINT REFERENCES owner_client_orders(id) ON DELETE CASCADE, raw_requirements TEXT NOT NULL, parsed_requirements JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_order_assets (id BIGSERIAL PRIMARY KEY, order_id BIGINT REFERENCES owner_client_orders(id) ON DELETE CASCADE, asset_type TEXT NOT NULL, asset_url TEXT, metadata JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_delivery_packages (id BIGSERIAL PRIMARY KEY, order_id BIGINT REFERENCES owner_client_orders(id) ON DELETE CASCADE, delivery_message TEXT, checklist JSONB NOT NULL DEFAULT '[]'::jsonb, delivered_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_revision_requests (id BIGSERIAL PRIMARY KEY, order_id BIGINT REFERENCES owner_client_orders(id) ON DELETE CASCADE, request_text TEXT NOT NULL, response_draft TEXT, status TEXT NOT NULL DEFAULT 'open', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_service_templates (id BIGSERIAL PRIMARY KEY, owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, name TEXT NOT NULL, template JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE IF NOT EXISTS owner_profit_records (id BIGSERIAL PRIMARY KEY, order_id BIGINT REFERENCES owner_client_orders(id) ON DELETE CASCADE, gross_amount NUMERIC(12,2) NOT NULL DEFAULT 0, platform_fees NUMERIC(12,2) NOT NULL DEFAULT 0, production_cost NUMERIC(12,2) NOT NULL DEFAULT 0, net_profit NUMERIC(12,2) GENERATED ALWAYS AS (gross_amount - platform_fees - production_cost) STORED, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
