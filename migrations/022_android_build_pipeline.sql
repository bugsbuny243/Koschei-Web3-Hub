ALTER TABLE generated_artifacts
  ADD COLUMN IF NOT EXISTS build_status text NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS base_bundle_size_mb numeric(10,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS asset_pack_size_mb numeric(10,2) NOT NULL DEFAULT 0;

ALTER TABLE generated_artifacts
  ADD CONSTRAINT generated_artifacts_build_status_ck
  CHECK (build_status IN ('pending','compiling','optimizing','ready','failed'));

CREATE TABLE IF NOT EXISTS runtime_build_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  artifact_id uuid NOT NULL REFERENCES generated_artifacts(id) ON DELETE CASCADE,
  runtime_log_id uuid REFERENCES runtime_logs(id) ON DELETE SET NULL,
  level text NOT NULL DEFAULT 'info',
  source text NOT NULL DEFAULT 'android_builder',
  message text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_runtime_build_logs_artifact ON runtime_build_logs(artifact_id, created_at DESC);
