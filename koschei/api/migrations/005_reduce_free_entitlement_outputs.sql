-- Reduce active free entitlements to the safer live-product quota while
-- preserving already used outputs.
--
-- used outputs = outputs_total - outputs_remaining
-- new remaining = max(10 - used outputs, 0)
UPDATE entitlements
SET
  outputs_remaining = GREATEST(10 - (outputs_total - outputs_remaining), 0),
  outputs_total = 10,
  updated_at = now()
WHERE status = 'active'
  AND COALESCE(plan_id, 'free') = 'free'
  AND outputs_total > 10;
