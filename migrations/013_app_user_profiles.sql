CREATE TABLE IF NOT EXISTS app_user_profiles (
  id uuid primary key default gen_random_uuid(),
  auth_subject text unique not null,
  email text unique not null,
  role text not null default 'free_user',
  plan_id text not null default 'free',
  credits integer not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

CREATE INDEX IF NOT EXISTS app_user_profiles_email_lower_idx
ON app_user_profiles (lower(email));
