CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS security_radar_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  module_id TEXT NOT NULL,
  label TEXT NOT NULL,
  address TEXT NOT NULL,
  network TEXT NOT NULL DEFAULT 'solana-mainnet',
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS module_id TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS label TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS address TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS network TEXT DEFAULT 'solana-mainnet';
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS enabled BOOLEAN DEFAULT true;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS target TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS target_type TEXT DEFAULT 'program';
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS provider TEXT DEFAULT 'alchemy';
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS watch_mode TEXT DEFAULT 'polling';
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS last_seen_signature TEXT;
ALTER TABLE security_radar_sources ADD COLUMN IF NOT EXISTS last_seen_slot BIGINT;

UPDATE security_radar_sources SET label = COALESCE(NULLIF(label,''), NULLIF(name,''), module_id) WHERE label IS NULL OR label = '';
UPDATE security_radar_sources SET address = COALESCE(NULLIF(address,''), NULLIF(target,'')) WHERE address IS NULL OR address = '';
UPDATE security_radar_sources SET network = 'solana-mainnet' WHERE network IS NULL OR network = '';
UPDATE security_radar_sources SET enabled = true WHERE enabled IS NULL;
UPDATE security_radar_sources SET created_at = now() WHERE created_at IS NULL;
UPDATE security_radar_sources SET updated_at = now() WHERE updated_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS security_radar_sources_unique_idx ON security_radar_sources (module_id, address, network);
CREATE INDEX IF NOT EXISTS security_radar_sources_enabled_idx ON security_radar_sources (enabled, updated_at DESC);
