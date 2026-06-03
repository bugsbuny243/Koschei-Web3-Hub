CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS web3_event_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text,
  label text,
  chain text,
  network text,
  address text,
  source_type text DEFAULT 'wallet',
  status text DEFAULT 'waiting_for_setup',
  provider text DEFAULT 'alchemy',
  setup_token text,
  webhook_url text,
  provider_setup_status text DEFAULT 'manual_required',
  last_event_at timestamptz,
  notes text,
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS email text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS label text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS chain text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS network text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS address text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS source_type text DEFAULT 'wallet';
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS status text DEFAULT 'waiting_for_setup';
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS provider text DEFAULT 'alchemy';
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS setup_token text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS webhook_url text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS provider_setup_status text DEFAULT 'manual_required';
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS last_event_at timestamptz;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS notes text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();

-- Legacy compatibility columns retained for older code paths/deployments.
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS user_id text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS name text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS is_active boolean DEFAULT true;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS secret_hash text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS verification_mode text;
ALTER TABLE web3_event_sources ADD COLUMN IF NOT EXISTS disabled_reason text;

ALTER TABLE web3_event_sources ALTER COLUMN source_type SET DEFAULT 'wallet';
ALTER TABLE web3_event_sources ALTER COLUMN status SET DEFAULT 'waiting_for_setup';
ALTER TABLE web3_event_sources ALTER COLUMN provider SET DEFAULT 'alchemy';
ALTER TABLE web3_event_sources ALTER COLUMN provider_setup_status SET DEFAULT 'manual_required';
ALTER TABLE web3_event_sources ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE web3_event_sources ALTER COLUMN updated_at SET DEFAULT now();

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='web3_event_sources' AND column_name='id' AND data_type='uuid'
  ) THEN
    ALTER TABLE web3_event_sources ALTER COLUMN id SET DEFAULT gen_random_uuid();
  ELSIF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='web3_event_sources' AND column_name='id'
  ) THEN
    ALTER TABLE web3_event_sources ALTER COLUMN id SET DEFAULT gen_random_uuid()::text;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS web3_event_sources_email_idx ON web3_event_sources (lower(email));
CREATE INDEX IF NOT EXISTS web3_event_sources_address_idx ON web3_event_sources (lower(address));
CREATE INDEX IF NOT EXISTS web3_event_sources_status_idx ON web3_event_sources (status);

CREATE TABLE IF NOT EXISTS web3_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid,
  email text,
  provider text DEFAULT 'alchemy',
  chain text,
  network text,
  event_type text,
  address text,
  tx_hash text,
  block_number text,
  direction text,
  asset_type text,
  amount text,
  raw_payload jsonb DEFAULT '{}'::jsonb,
  verification_status text DEFAULT 'unverified',
  status text DEFAULT 'received',
  created_at timestamptz DEFAULT now()
);

ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS source_id uuid;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS email text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS provider text DEFAULT 'alchemy';
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS chain text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS network text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS event_type text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS address text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS tx_hash text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS block_number text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS direction text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS asset_type text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS amount text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS raw_payload jsonb DEFAULT '{}'::jsonb;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS verification_status text DEFAULT 'unverified';
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS status text DEFAULT 'received';
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();

-- Old compatibility columns kept if existing clients still read them.
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS wallet_address text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS contract_address text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS amount_text text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS user_id text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS source_name text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS token_id text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS ai_summary text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS risk_level text DEFAULT 'unknown';
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS payload_hash text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS error_message text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS received_ip text;
ALTER TABLE web3_events ADD COLUMN IF NOT EXISTS received_user_agent text;

ALTER TABLE web3_events ALTER COLUMN provider SET DEFAULT 'alchemy';
ALTER TABLE web3_events ALTER COLUMN raw_payload SET DEFAULT '{}'::jsonb;
ALTER TABLE web3_events ALTER COLUMN verification_status SET DEFAULT 'unverified';
ALTER TABLE web3_events ALTER COLUMN status SET DEFAULT 'received';
ALTER TABLE web3_events ALTER COLUMN created_at SET DEFAULT now();

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='web3_events' AND column_name='id' AND data_type='uuid'
  ) THEN
    ALTER TABLE web3_events ALTER COLUMN id SET DEFAULT gen_random_uuid();
  ELSIF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name='web3_events' AND column_name='id'
  ) THEN
    ALTER TABLE web3_events ALTER COLUMN id SET DEFAULT gen_random_uuid()::text;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS web3_events_email_created_idx ON web3_events (lower(email), created_at DESC);
CREATE INDEX IF NOT EXISTS web3_events_source_created_idx ON web3_events (source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS web3_events_tx_hash_idx ON web3_events (tx_hash);
