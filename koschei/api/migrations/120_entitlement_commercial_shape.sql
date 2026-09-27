-- Complete the commercial entitlement shape expected by the split-plane runtime.
-- Forward-only migration: do not edit historical migrations to backfill these
-- columns because deployed databases may already have recorded them as applied.

ALTER TABLE entitlements
    ADD COLUMN IF NOT EXISTS starts_at TIMESTAMPTZ DEFAULT now(),
    ADD COLUMN IF NOT EXISTS order_id UUID;

UPDATE entitlements
SET starts_at = COALESCE(starts_at, created_at, now())
WHERE starts_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_entitlements_active_plan_window
    ON entitlements (lower(email), plan_id, status, starts_at, expires_at);

COMMENT ON COLUMN entitlements.starts_at IS
    'Commercial entitlement activation boundary used by plan authorization.';
COMMENT ON COLUMN entitlements.order_id IS
    'Optional provider/order correlation identifier; never an authorization source.';
