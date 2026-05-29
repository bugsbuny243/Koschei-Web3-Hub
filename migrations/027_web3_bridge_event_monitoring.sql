-- Koschei Web3 Bridge MVP event monitoring foundation.
-- Read-only monitoring only: no private keys, custody, escrow, or automatic transfers.

CREATE TABLE IF NOT EXISTS web3_event_sources (
  id uuid PRIMARY KEY,
  user_id text,
  name text NOT NULL,
  provider text NOT NULL DEFAULT 'alchemy',
  network text NOT NULL,
  webhook_url text,
  is_active boolean DEFAULT true,
  metadata jsonb DEFAULT '{}'::jsonb,
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS web3_events (
  id uuid PRIMARY KEY,
  source_id uuid REFERENCES web3_event_sources(id),
  user_id text,
  provider text NOT NULL DEFAULT 'alchemy',
  network text,
  event_type text,
  tx_hash text,
  wallet_address text,
  contract_address text,
  token_id text,
  amount_text text,
  raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  ai_summary text,
  risk_level text DEFAULT 'unknown',
  status text DEFAULT 'received',
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS web3_event_notes (
  id uuid PRIMARY KEY,
  event_id uuid NOT NULL REFERENCES web3_events(id) ON DELETE CASCADE,
  user_id text,
  note text NOT NULL,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS web3_alerts (
  id uuid PRIMARY KEY,
  event_id uuid REFERENCES web3_events(id) ON DELETE CASCADE,
  user_id text,
  alert_type text,
  message text,
  status text DEFAULT 'open',
  created_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_web3_event_sources_user_id ON web3_event_sources(user_id);
CREATE INDEX IF NOT EXISTS idx_web3_event_sources_created_at ON web3_event_sources(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_web3_event_sources_network ON web3_event_sources(network);

CREATE INDEX IF NOT EXISTS idx_web3_events_user_id ON web3_events(user_id);
CREATE INDEX IF NOT EXISTS idx_web3_events_created_at ON web3_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_web3_events_tx_hash ON web3_events(tx_hash);
CREATE INDEX IF NOT EXISTS idx_web3_events_event_type ON web3_events(event_type);
CREATE INDEX IF NOT EXISTS idx_web3_events_network ON web3_events(network);
CREATE INDEX IF NOT EXISTS idx_web3_events_user_created_at ON web3_events(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_web3_event_notes_user_id ON web3_event_notes(user_id);
CREATE INDEX IF NOT EXISTS idx_web3_event_notes_created_at ON web3_event_notes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_web3_event_notes_event_id ON web3_event_notes(event_id);

CREATE INDEX IF NOT EXISTS idx_web3_alerts_user_id ON web3_alerts(user_id);
CREATE INDEX IF NOT EXISTS idx_web3_alerts_created_at ON web3_alerts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_web3_alerts_event_id ON web3_alerts(event_id);
