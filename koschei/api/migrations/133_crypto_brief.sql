-- Customer opt-in news notifications; independent of TradePI agent channels and ARVIS evidence.
CREATE TABLE IF NOT EXISTS crypto_brief_sources (
  source_id text PRIMARY KEY, checked_at timestamptz, success_at timestamptz,
  error_code text NOT NULL DEFAULT '', item_count integer NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS crypto_brief_items (
  id text PRIMARY KEY CHECK (length(id)=64), source_id text NOT NULL,
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 240),
  url text NOT NULL CHECK (length(url)<=2048), published_at timestamptz NOT NULL,
  discovered_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  networks text[] NOT NULL, topics text[] NOT NULL
);
CREATE INDEX IF NOT EXISTS crypto_brief_items_recent ON crypto_brief_items(published_at DESC, id);
CREATE TABLE IF NOT EXISTS crypto_brief_subscriptions (
  id bigserial PRIMARY KEY, customer_sub text NOT NULL CHECK (length(customer_sub) BETWEEN 1 AND 256),
  channel text NOT NULL CHECK (channel IN ('telegram','whatsapp')),
  recipient text, state text NOT NULL DEFAULT 'disconnected' CHECK (state IN ('disconnected','active','paused')),
  networks text[] NOT NULL DEFAULT '{}', topics text[] NOT NULL DEFAULT '{}',
  cadence text NOT NULL DEFAULT 'daily' CHECK (cadence IN ('daily','hourly')),
  timezone text NOT NULL DEFAULT 'Europe/Istanbul',
  quiet_start smallint NOT NULL DEFAULT 23 CHECK (quiet_start BETWEEN 0 AND 23),
  quiet_end smallint NOT NULL DEFAULT 8 CHECK (quiet_end BETWEEN 0 AND 23),
  next_digest_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  last_digest_at timestamptz NOT NULL DEFAULT clock_timestamp()-interval '24 hours',
  last_inbound_at timestamptz, consent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE(customer_sub,channel), CHECK (state='disconnected' OR (recipient IS NOT NULL AND consent_at IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS crypto_brief_recipient ON crypto_brief_subscriptions(channel,recipient) WHERE recipient IS NOT NULL;
CREATE INDEX IF NOT EXISTS crypto_brief_due ON crypto_brief_subscriptions(next_digest_at,id) WHERE state='active';
CREATE TABLE IF NOT EXISTS crypto_brief_pairings (
  customer_sub text NOT NULL, channel text NOT NULL CHECK (channel IN ('telegram','whatsapp')),
  token_hash text NOT NULL UNIQUE CHECK (length(token_hash)=64),
  expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY(customer_sub,channel)
);
CREATE TABLE IF NOT EXISTS crypto_brief_inbound (
  channel text NOT NULL, event_id text NOT NULL CHECK (length(event_id)<=256),
  received_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(channel,event_id)
);
CREATE TABLE IF NOT EXISTS crypto_brief_deliveries (
  id bigserial PRIMARY KEY, subscription_id bigint NOT NULL REFERENCES crypto_brief_subscriptions(id) ON DELETE CASCADE,
  dedup_key text NOT NULL CHECK (length(dedup_key)<=300),
  body text NOT NULL CHECK (length(body) BETWEEN 1 AND 3500),
  state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','sending','accepted','delivered','uncertain','failed','cancelled')),
  due_at timestamptz NOT NULL DEFAULT clock_timestamp(), lease_until timestamptz,
  fencing_token bigint NOT NULL DEFAULT 0, attempts integer NOT NULL DEFAULT 0,
  provider_id text, reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE(subscription_id,dedup_key)
);
CREATE INDEX IF NOT EXISTS crypto_brief_delivery_due ON crypto_brief_deliveries(due_at,id) WHERE state='pending';
CREATE INDEX IF NOT EXISTS crypto_brief_delivery_lease ON crypto_brief_deliveries(lease_until) WHERE state='sending';
CREATE INDEX IF NOT EXISTS crypto_brief_provider_receipt ON crypto_brief_deliveries(provider_id) WHERE provider_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS crypto_brief_seen (
  subscription_id bigint NOT NULL REFERENCES crypto_brief_subscriptions(id) ON DELETE CASCADE,
  item_id text NOT NULL REFERENCES crypto_brief_items(id) ON DELETE CASCADE,
  seen_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(subscription_id,item_id)
);
