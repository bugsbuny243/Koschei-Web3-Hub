CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS web3_event_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT,
  chain TEXT,
  network TEXT,
  address TEXT,
  label TEXT,
  source_type TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE web3_event_sources
  ADD COLUMN IF NOT EXISTS email TEXT,
  ADD COLUMN IF NOT EXISTS chain TEXT,
  ADD COLUMN IF NOT EXISTS network TEXT,
  ADD COLUMN IF NOT EXISTS address TEXT,
  ADD COLUMN IF NOT EXISTS label TEXT,
  ADD COLUMN IF NOT EXISTS source_type TEXT,
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS notes TEXT,
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS user_id TEXT,
  ADD COLUMN IF NOT EXISTS name TEXT,
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'alchemy',
  ADD COLUMN IF NOT EXISTS webhook_url TEXT,
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS secret_hash TEXT,
  ADD COLUMN IF NOT EXISTS verification_mode TEXT NOT NULL DEFAULT 'alchemy_signature',
  ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS disabled_reason TEXT;

UPDATE web3_event_sources
SET
  email = COALESCE(email, user_id),
  label = COALESCE(label, name),
  status = COALESCE(status, CASE WHEN COALESCE(is_active, true) THEN 'active' ELSE 'inactive' END),
  source_type = COALESCE(source_type, 'wallet'),
  chain = COALESCE(chain, split_part(network, '-', 1)),
  updated_at = now()
WHERE email IS NULL OR label IS NULL OR status IS NULL OR source_type IS NULL OR chain IS NULL;

CREATE INDEX IF NOT EXISTS web3_event_sources_email_idx ON web3_event_sources (lower(email));
CREATE INDEX IF NOT EXISTS web3_event_sources_address_idx ON web3_event_sources (lower(address));
CREATE INDEX IF NOT EXISTS web3_event_sources_status_idx ON web3_event_sources (status);
CREATE INDEX IF NOT EXISTS web3_event_sources_active_address_idx ON web3_event_sources (lower(address)) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS web3_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID NULL,
  email TEXT NULL,
  chain TEXT,
  network TEXT,
  event_type TEXT,
  address TEXT,
  tx_hash TEXT,
  block_number TEXT NULL,
  direction TEXT NULL,
  asset_type TEXT NULL,
  amount TEXT NULL,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE web3_events
  ADD COLUMN IF NOT EXISTS source_id UUID NULL,
  ADD COLUMN IF NOT EXISTS email TEXT NULL,
  ADD COLUMN IF NOT EXISTS chain TEXT,
  ADD COLUMN IF NOT EXISTS network TEXT,
  ADD COLUMN IF NOT EXISTS event_type TEXT,
  ADD COLUMN IF NOT EXISTS address TEXT,
  ADD COLUMN IF NOT EXISTS tx_hash TEXT,
  ADD COLUMN IF NOT EXISTS block_number TEXT NULL,
  ADD COLUMN IF NOT EXISTS direction TEXT NULL,
  ADD COLUMN IF NOT EXISTS asset_type TEXT NULL,
  ADD COLUMN IF NOT EXISTS amount TEXT NULL,
  ADD COLUMN IF NOT EXISTS raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS source_name TEXT,
  ADD COLUMN IF NOT EXISTS user_id TEXT,
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'alchemy',
  ADD COLUMN IF NOT EXISTS wallet_address TEXT,
  ADD COLUMN IF NOT EXISTS contract_address TEXT,
  ADD COLUMN IF NOT EXISTS token_id TEXT,
  ADD COLUMN IF NOT EXISTS amount_text TEXT,
  ADD COLUMN IF NOT EXISTS ai_summary TEXT,
  ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'unknown',
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'received',
  ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'verified',
  ADD COLUMN IF NOT EXISTS received_ip TEXT,
  ADD COLUMN IF NOT EXISTS received_user_agent TEXT,
  ADD COLUMN IF NOT EXISTS payload_hash TEXT,
  ADD COLUMN IF NOT EXISTS error_message TEXT;

UPDATE web3_events
SET
  email = COALESCE(email, user_id),
  address = COALESCE(address, wallet_address, contract_address),
  amount = COALESCE(amount, amount_text)
WHERE email IS NULL OR address IS NULL OR amount IS NULL;

CREATE INDEX IF NOT EXISTS web3_events_email_created_at_idx ON web3_events (lower(email), created_at DESC);
CREATE INDEX IF NOT EXISTS web3_events_source_created_at_idx ON web3_events (source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS web3_events_tx_hash_idx ON web3_events (tx_hash);
