CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS auth_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL,
  password_hash text NOT NULL,
  role text NOT NULL DEFAULT 'user',
  plan_id text NOT NULL DEFAULT 'free',
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS id uuid DEFAULT gen_random_uuid();
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS email text;
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS password_hash text;
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS role text DEFAULT 'user';
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS plan_id text DEFAULT 'free';
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS is_active boolean DEFAULT true;
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();
ALTER TABLE auth_accounts ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();

UPDATE auth_accounts SET role='user' WHERE role IS NULL;
UPDATE auth_accounts SET plan_id='free' WHERE plan_id IS NULL OR plan_id='';
UPDATE auth_accounts SET is_active=true WHERE is_active IS NULL;
UPDATE auth_accounts SET created_at=now() WHERE created_at IS NULL;
UPDATE auth_accounts SET updated_at=now() WHERE updated_at IS NULL;

ALTER TABLE auth_accounts ALTER COLUMN email SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN role SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN plan_id SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN is_active SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE auth_accounts ALTER COLUMN updated_at SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS auth_accounts_email_lower_unique ON auth_accounts (lower(email));
CREATE INDEX IF NOT EXISTS auth_accounts_role_idx ON auth_accounts (role);
CREATE INDEX IF NOT EXISTS auth_accounts_plan_id_idx ON auth_accounts (plan_id);
