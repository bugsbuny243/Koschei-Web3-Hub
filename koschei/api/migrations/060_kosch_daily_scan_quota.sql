ALTER TABLE entitlements
  ADD COLUMN IF NOT EXISTS quota_day date,
  ADD COLUMN IF NOT EXISTS quota_tier text,
  ADD COLUMN IF NOT EXISTS quota_kind text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'entitlements_quota_tier_check'
      AND conrelid = 'entitlements'::regclass
  ) THEN
    ALTER TABLE entitlements
      ADD CONSTRAINT entitlements_quota_tier_check
      CHECK (quota_tier IS NULL OR quota_tier IN ('basic','pro','enterprise'));
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_entitlements_daily_quota_unique
  ON entitlements (lower(email), quota_kind, quota_day)
  WHERE quota_day IS NOT NULL AND quota_kind IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_entitlements_daily_quota_active
  ON entitlements (lower(email), quota_day, quota_kind, status, expires_at)
  WHERE quota_day IS NOT NULL;
