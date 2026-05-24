-- Phase 4.1 schema repair: align AI tables with canonical handler usage

ALTER TABLE generation_jobs
  ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE model_route_logs
  ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE credit_events
  ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE generation_jobs
  ADD COLUMN IF NOT EXISTS tool text,
  ADD COLUMN IF NOT EXISTS provider text,
  ADD COLUMN IF NOT EXISTS route text,
  ADD COLUMN IF NOT EXISTS result text,
  ADD COLUMN IF NOT EXISTS error text,
  ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE model_route_logs
  ADD COLUMN IF NOT EXISTS tool text,
  ADD COLUMN IF NOT EXISTS provider text,
  ADD COLUMN IF NOT EXISTS prompt text,
  ADD COLUMN IF NOT EXISTS status text,
  ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE credit_events
  ADD COLUMN IF NOT EXISTS event_type text NOT NULL DEFAULT 'adjustment',
  ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();
