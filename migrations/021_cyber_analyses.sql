CREATE TABLE IF NOT EXISTS cyber_analyses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_subject text,
  user_email text,
  mode text NOT NULL,
  prompt text NOT NULL,
  status text NOT NULL DEFAULT 'completed',
  model text,
  result jsonb NOT NULL DEFAULT '{}'::jsonb,
  error_message text,
  credits_charged boolean NOT NULL DEFAULT false,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cyber_analyses_auth_subject_created_at
  ON cyber_analyses (auth_subject, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cyber_analyses_user_email_created_at
  ON cyber_analyses (user_email, created_at DESC);
