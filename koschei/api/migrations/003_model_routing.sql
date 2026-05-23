ALTER TABLE generation_jobs
  ADD COLUMN IF NOT EXISTS route text,
  ADD COLUMN IF NOT EXISTS provider text;

UPDATE generation_jobs
SET route = COALESCE(route, 'chat_analysis'),
    provider = COALESCE(provider, 'together')
WHERE route IS NULL OR provider IS NULL;

ALTER TABLE generation_jobs
  ALTER COLUMN route SET DEFAULT 'chat_analysis',
  ALTER COLUMN provider SET DEFAULT 'together';

CREATE TABLE IF NOT EXISTS model_route_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text,
  tool text NOT NULL,
  route text NOT NULL,
  provider text NOT NULL,
  prompt text,
  status text NOT NULL DEFAULT 'mock',
  created_at timestamptz NOT NULL DEFAULT now()
);
