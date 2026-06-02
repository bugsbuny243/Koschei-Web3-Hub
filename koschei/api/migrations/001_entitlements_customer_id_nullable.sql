ALTER TABLE entitlements
  ALTER COLUMN customer_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS entitlements_email_idx ON entitlements (email);
CREATE INDEX IF NOT EXISTS entitlements_status_idx ON entitlements (status);
