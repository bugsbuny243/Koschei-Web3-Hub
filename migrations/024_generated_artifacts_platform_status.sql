ALTER TABLE generated_artifacts
  ADD COLUMN IF NOT EXISTS target_platform text DEFAULT 'android',
  ADD COLUMN IF NOT EXISTS base_bundle_size_mb numeric DEFAULT 0,
  ADD COLUMN IF NOT EXISTS asset_pack_size_mb numeric DEFAULT 0,
  ADD COLUMN IF NOT EXISTS build_status text DEFAULT 'pending';

UPDATE generated_artifacts
SET
  target_platform = COALESCE(target_platform, 'android'),
  base_bundle_size_mb = COALESCE(base_bundle_size_mb, 0),
  asset_pack_size_mb = COALESCE(asset_pack_size_mb, 0),
  build_status = COALESCE(build_status, 'pending')
WHERE
  target_platform IS NULL
  OR base_bundle_size_mb IS NULL
  OR asset_pack_size_mb IS NULL
  OR build_status IS NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'generated_artifacts_target_platform_check'
  ) THEN
    ALTER TABLE generated_artifacts
      ADD CONSTRAINT generated_artifacts_target_platform_check
      CHECK (target_platform IN ('android', 'pc', 'web', 'unknown'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'generated_artifacts_build_status_check'
  ) THEN
    ALTER TABLE generated_artifacts
      ADD CONSTRAINT generated_artifacts_build_status_check
      CHECK (build_status IN ('pending', 'queued', 'building', 'optimizing', 'packaged', 'uploading', 'uploaded', 'failed'));
  END IF;
END $$;
