CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject text NOT NULL,
    name text NOT NULL,
    url text NOT NULL,
    secret_ciphertext text NOT NULL,
    secret_last4 text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    event_types text[] NOT NULL DEFAULT ARRAY['watchlist.alert.created']::text[],
    failure_count integer NOT NULL DEFAULT 0,
    last_delivery_at timestamptz,
    last_success_at timestamptz,
    last_failure_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_endpoints_status_check CHECK (status IN ('active','paused')),
    CONSTRAINT webhook_endpoints_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    UNIQUE (auth_subject, name)
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_owner_updated
    ON webhook_endpoints (auth_subject, updated_at DESC);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    auth_subject text NOT NULL,
    event_id uuid,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempt_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 6,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    last_http_status integer,
    last_error text,
    response_excerpt text,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('pending','processing','retry','delivered','dead_letter')),
    CONSTRAINT webhook_deliveries_attempt_check CHECK (attempt_count >= 0 AND max_attempts BETWEEN 1 AND 12),
    UNIQUE (endpoint_id, event_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due
    ON webhook_deliveries (status, next_attempt_at, created_at)
    WHERE status IN ('pending','retry');
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_owner_created
    ON webhook_deliveries (auth_subject, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint_created
    ON webhook_deliveries (endpoint_id, created_at DESC);
