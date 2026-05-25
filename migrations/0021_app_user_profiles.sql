CREATE TABLE IF NOT EXISTS app_user_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_subject text UNIQUE NOT NULL,
  email text UNIQUE NOT NULL,
  role text NOT NULL DEFAULT 'free_user',
  plan_id text NOT NULL DEFAULT 'free',
  credits integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS app_user_profiles_email_idx
ON app_user_profiles (lower(email));
