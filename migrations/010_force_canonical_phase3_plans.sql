ALTER TABLE plans
ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE plans
ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

INSERT INTO plans (id, name, price_try, monthly_credits, is_active)
VALUES
  ('free', 'Free', 0, 0, true),
  ('starter', 'Starter', 899, 20000, true),
  ('pro', 'Pro', 2299, 70000, true),
  ('studio', 'Studio', 4999, 180000, true)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  price_try = EXCLUDED.price_try,
  monthly_credits = EXCLUDED.monthly_credits,
  is_active = EXCLUDED.is_active,
  updated_at = now();
