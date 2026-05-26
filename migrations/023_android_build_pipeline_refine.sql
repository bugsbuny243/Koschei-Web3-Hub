ALTER TABLE generated_artifacts
  ADD COLUMN IF NOT EXISTS target_platform VARCHAR(50) NOT NULL DEFAULT 'android',
  ADD COLUMN IF NOT EXISTS base_bundle_size_mb NUMERIC(6,2),
  ADD COLUMN IF NOT EXISTS asset_pack_size_mb NUMERIC(6,2),
  ADD COLUMN IF NOT EXISTS build_status VARCHAR(50) NOT NULL DEFAULT 'pending';
