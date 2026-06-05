-- Cancel all complimentary free outputs. Paid entitlements are intentionally untouched.
UPDATE entitlements
SET outputs_total = 0,
    outputs_remaining = 0,
    updated_at = now()
WHERE COALESCE(plan_id, 'free') = 'free'
  AND status = 'active';

UPDATE plans
SET monthly_credits = 0,
    updated_at = now()
WHERE id = 'free';
