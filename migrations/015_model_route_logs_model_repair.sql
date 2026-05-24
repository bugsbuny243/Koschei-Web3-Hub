ALTER TABLE model_route_logs
  ADD COLUMN IF NOT EXISTS model text;

UPDATE model_route_logs
SET model = COALESCE(NULLIF(model, ''), route, provider, 'unknown')
WHERE model IS NULL OR model = '';

ALTER TABLE model_route_logs
  ALTER COLUMN model SET DEFAULT 'unknown';
