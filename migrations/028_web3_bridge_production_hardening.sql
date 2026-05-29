-- Koschei Web3 Bridge production hardening.
-- Read-only monitoring only: no private keys, custody, escrow, or automatic transfers.

ALTER TABLE web3_event_sources
  ADD COLUMN IF NOT EXISTS secret_hash text,
  ADD COLUMN IF NOT EXISTS allowed_ip_ranges text[],
  ADD COLUMN IF NOT EXISTS verification_mode text NOT NULL DEFAULT 'shared_secret',
  ADD COLUMN IF NOT EXISTS last_event_at timestamptz,
  ADD COLUMN IF NOT EXISTS disabled_reason text;

ALTER TABLE web3_events
  ADD COLUMN IF NOT EXISTS source_name text,
  ADD COLUMN IF NOT EXISTS verification_status text NOT NULL DEFAULT 'unverified',
  ADD COLUMN IF NOT EXISTS received_ip text,
  ADD COLUMN IF NOT EXISTS received_user_agent text,
  ADD COLUMN IF NOT EXISTS payload_hash text,
  ADD COLUMN IF NOT EXISTS error_message text;

CREATE INDEX IF NOT EXISTS idx_web3_events_verification_status ON web3_events(verification_status);
CREATE INDEX IF NOT EXISTS idx_web3_events_payload_hash ON web3_events(payload_hash);
CREATE INDEX IF NOT EXISTS idx_web3_event_sources_provider_network_active ON web3_event_sources(provider, network, is_active);
