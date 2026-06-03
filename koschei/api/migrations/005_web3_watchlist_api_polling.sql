CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS web3_event_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT,
  email TEXT,
  name TEXT NOT NULL DEFAULT '',
  label TEXT,
  provider TEXT NOT NULL DEFAULT 'alchemy',
  chain TEXT,
  network TEXT NOT NULL DEFAULT 'base-mainnet',
  address TEXT,
  source_type TEXT NOT NULL DEFAULT 'wallet',
  notes TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  provider_setup_status TEXT NOT NULL DEFAULT 'api_polling',
  webhook_url TEXT,
  is_active BOOLEAN NOT NULL DEFAULT true,
  secret_hash TEXT,
  verification_mode TEXT NOT NULL DEFAULT 'api_polling',
  last_event_at TIMESTAMPTZ,
  disabled_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE web3_event_sources
  ADD COLUMN IF NOT EXISTS user_id TEXT,
  ADD COLUMN IF NOT EXISTS email TEXT,
  ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS label TEXT,
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'alchemy',
  ADD COLUMN IF NOT EXISTS chain TEXT,
  ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'base-mainnet',
  ADD COLUMN IF NOT EXISTS address TEXT,
  ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'wallet',
  ADD COLUMN IF NOT EXISTS notes TEXT,
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS provider_setup_status TEXT NOT NULL DEFAULT 'api_polling',
  ADD COLUMN IF NOT EXISTS webhook_url TEXT,
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS secret_hash TEXT,
  ADD COLUMN IF NOT EXISTS verification_mode TEXT NOT NULL DEFAULT 'api_polling',
  ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS disabled_reason TEXT,
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS web3_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID,
  source_name TEXT,
  user_id TEXT,
  email TEXT,
  provider TEXT NOT NULL DEFAULT 'alchemy',
  chain TEXT,
  network TEXT,
  event_type TEXT,
  address TEXT,
  tx_hash TEXT,
  block_number TEXT,
  direction TEXT,
  asset_type TEXT,
  amount TEXT,
  wallet_address TEXT,
  contract_address TEXT,
  token_id TEXT,
  amount_text TEXT,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  ai_summary TEXT,
  risk_level TEXT NOT NULL DEFAULT 'unknown',
  status TEXT NOT NULL DEFAULT 'received',
  verification_status TEXT NOT NULL DEFAULT 'api_polling',
  received_ip TEXT,
  received_user_agent TEXT,
  payload_hash TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE web3_events
  ADD COLUMN IF NOT EXISTS source_id UUID,
  ADD COLUMN IF NOT EXISTS source_name TEXT,
  ADD COLUMN IF NOT EXISTS user_id TEXT,
  ADD COLUMN IF NOT EXISTS email TEXT,
  ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'alchemy',
  ADD COLUMN IF NOT EXISTS chain TEXT,
  ADD COLUMN IF NOT EXISTS network TEXT,
  ADD COLUMN IF NOT EXISTS event_type TEXT,
  ADD COLUMN IF NOT EXISTS address TEXT,
  ADD COLUMN IF NOT EXISTS tx_hash TEXT,
  ADD COLUMN IF NOT EXISTS block_number TEXT,
  ADD COLUMN IF NOT EXISTS direction TEXT,
  ADD COLUMN IF NOT EXISTS asset_type TEXT,
  ADD COLUMN IF NOT EXISTS amount TEXT,
  ADD COLUMN IF NOT EXISTS wallet_address TEXT,
  ADD COLUMN IF NOT EXISTS contract_address TEXT,
  ADD COLUMN IF NOT EXISTS token_id TEXT,
  ADD COLUMN IF NOT EXISTS amount_text TEXT,
  ADD COLUMN IF NOT EXISTS raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS ai_summary TEXT,
  ADD COLUMN IF NOT EXISTS risk_level TEXT NOT NULL DEFAULT 'unknown',
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'received',
  ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'api_polling',
  ADD COLUMN IF NOT EXISTS received_ip TEXT,
  ADD COLUMN IF NOT EXISTS received_user_agent TEXT,
  ADD COLUMN IF NOT EXISTS payload_hash TEXT,
  ADD COLUMN IF NOT EXISTS error_message TEXT,
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS web3_event_sources_email_idx ON web3_event_sources (lower(email));
CREATE INDEX IF NOT EXISTS web3_event_sources_email_status_idx ON web3_event_sources (lower(email), status);
CREATE INDEX IF NOT EXISTS web3_events_email_created_at_idx ON web3_events (lower(email), created_at DESC);
CREATE INDEX IF NOT EXISTS web3_events_source_id_idx ON web3_events (source_id);
CREATE UNIQUE INDEX IF NOT EXISTS web3_events_polling_dedupe_idx
  ON web3_events (lower(email), lower(address), tx_hash, event_type)
  WHERE email IS NOT NULL AND address IS NOT NULL AND tx_hash IS NOT NULL AND event_type IS NOT NULL;
