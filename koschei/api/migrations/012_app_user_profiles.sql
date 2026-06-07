-- Migration: Neon Auth-backed customer profiles
CREATE TABLE IF NOT EXISTS app_user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    plan_id TEXT NOT NULL DEFAULT 'free',
    credits INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_user_profiles_email ON app_user_profiles(lower(email));
CREATE INDEX IF NOT EXISTS idx_app_user_profiles_role ON app_user_profiles(role);
