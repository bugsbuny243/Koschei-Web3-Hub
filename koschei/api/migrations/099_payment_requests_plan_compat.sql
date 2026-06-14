-- Compatibility for older payment_requests schemas that contain a NOT NULL plan column.
-- New Shopier requests store the selected package as product_slug/raw_payload.product_id.
-- This migration keeps the legacy plan column populated without making application code
-- depend on that legacy column.

ALTER TABLE payment_requests
  ADD COLUMN IF NOT EXISTS plan text;

UPDATE payment_requests
SET plan = COALESCE(
  NULLIF(plan, ''),
  NULLIF(product_slug, ''),
  raw_payload->>'product_id',
  'starter'
)
WHERE plan IS NULL OR plan = '';

ALTER TABLE payment_requests
  ALTER COLUMN plan SET DEFAULT 'starter';

ALTER TABLE payment_requests
  ALTER COLUMN plan SET NOT NULL;

CREATE OR REPLACE FUNCTION koschei_payment_requests_plan_sync()
RETURNS trigger AS $$
BEGIN
  NEW.plan := COALESCE(
    NULLIF(NEW.plan, ''),
    NULLIF(NEW.product_slug, ''),
    NEW.raw_payload->>'product_id',
    'starter'
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS payment_requests_plan_sync_trigger ON payment_requests;

CREATE TRIGGER payment_requests_plan_sync_trigger
BEFORE INSERT OR UPDATE ON payment_requests
FOR EACH ROW
EXECUTE FUNCTION koschei_payment_requests_plan_sync();
