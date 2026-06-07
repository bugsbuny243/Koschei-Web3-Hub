CREATE TABLE IF NOT EXISTS credit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  amount integer NOT NULL,
  reason text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_credit_events_email_created_at ON credit_events (lower(email), created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_events_reason_created_at ON credit_events (reason, created_at DESC);
