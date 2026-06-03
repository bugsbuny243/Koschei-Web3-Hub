CREATE TABLE IF NOT EXISTS analytics_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_name text NOT NULL,
  email text,
  path text NOT NULL,
  referrer text NOT NULL,
  user_agent text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS analytics_events_event_name_created_at_idx ON analytics_events (event_name, created_at DESC);
CREATE INDEX IF NOT EXISTS analytics_events_email_created_at_idx ON analytics_events (lower(email), created_at DESC) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS analytics_events_created_at_idx ON analytics_events (created_at DESC);
