CREATE TABLE IF NOT EXISTS global_campaign_worker_leases (
    lease_key TEXT PRIMARY KEY,
    lease_owner TEXT NOT NULL,
    fencing_token BIGINT NOT NULL CHECK (fencing_token > 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_global_campaign_worker_leases_expires_at
    ON global_campaign_worker_leases (lease_expires_at);

COMMENT ON TABLE global_campaign_worker_leases IS
    'Fenced Global Campaign worker leases. A stale worker must not commit authoritative campaign runtime work after lease takeover.';
