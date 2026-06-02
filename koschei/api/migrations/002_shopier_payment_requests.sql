ALTER TABLE payment_requests
  ADD COLUMN IF NOT EXISTS full_name text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS product_id text,
  ADD COLUMN IF NOT EXISTS amount_try integer,
  ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'TRY',
  ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS reviewed_at timestamptz;

CREATE INDEX IF NOT EXISTS payment_requests_created_at_idx ON payment_requests (created_at DESC);
CREATE INDEX IF NOT EXISTS payment_requests_status_idx ON payment_requests (status);

UPDATE payment_requests
SET product_id = COALESCE(product_id, plan),
    amount_try = COALESCE(amount_try, CASE COALESCE(product_id, plan)
      WHEN 'starter' THEN 899
      WHEN 'builder' THEN 2299
      WHEN 'studio' THEN 4999
      ELSE NULL
    END)
WHERE product_id IS NULL OR amount_try IS NULL;
