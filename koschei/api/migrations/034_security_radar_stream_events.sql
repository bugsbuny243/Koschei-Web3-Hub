CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS security_radar_stream_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL DEFAULT 'solana_wss',
  stream_mode TEXT NOT NULL DEFAULT 'logs_subscribe',
  network TEXT NOT NULL DEFAULT 'solana-mainnet',
  module_id TEXT NOT NULL DEFAULT 'unknown',
  event_type TEXT NOT NULL DEFAULT 'stream_event',
  target TEXT,
  target_type TEXT NOT NULL DEFAULT 'unknown',
  signature TEXT,
  slot BIGINT,
  program_id TEXT,
  evidence_quality TEXT NOT NULL DEFAULT 'raw_stream',
  decoded JSONB NOT NULL DEFAULT '{}'::jsonb,
  raw_event JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'solana_wss';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS stream_mode TEXT NOT NULL DEFAULT 'logs_subscribe';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS network TEXT NOT NULL DEFAULT 'solana-mainnet';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS module_id TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS event_type TEXT NOT NULL DEFAULT 'stream_event';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS target TEXT;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS target_type TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS signature TEXT;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS slot BIGINT;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS program_id TEXT;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS evidence_quality TEXT NOT NULL DEFAULT 'raw_stream';
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS decoded JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS raw_event JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE IF EXISTS security_radar_stream_events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS security_radar_stream_events_created_idx ON security_radar_stream_events (created_at DESC);
CREATE INDEX IF NOT EXISTS security_radar_stream_events_module_created_idx ON security_radar_stream_events (module_id, created_at DESC);
CREATE INDEX IF NOT EXISTS security_radar_stream_events_target_idx ON security_radar_stream_events (target, created_at DESC) WHERE target IS NOT NULL;
CREATE INDEX IF NOT EXISTS security_radar_stream_events_signature_idx ON security_radar_stream_events (signature) WHERE signature IS NOT NULL;
CREATE INDEX IF NOT EXISTS security_radar_stream_events_slot_idx ON security_radar_stream_events (slot DESC) WHERE slot IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS security_radar_stream_events_dedupe_idx
  ON security_radar_stream_events (COALESCE(signature,''), COALESCE(program_id,''), module_id, event_type, COALESCE(target,''))
  WHERE signature IS NOT NULL;
