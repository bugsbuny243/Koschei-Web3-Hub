-- Persist before ClickHouse publication. Acknowledgement and campaign enqueue
-- commit together; expired leases recover interrupted publication automatically.
CREATE TABLE IF NOT EXISTS global_radar_campaign_handoffs (
    source_ref TEXT PRIMARY KEY CHECK (source_ref ~ '^sha256:[0-9a-f]{64}$'),
    snapshot_payload JSONB,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','publishing','complete','failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_owner TEXT NOT NULL DEFAULT '',
    fencing_token BIGINT NOT NULL DEFAULT 0 CHECK (fencing_token >= 0),
    lease_until TIMESTAMPTZ,
    last_error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK ((status = 'publishing') = (lease_until IS NOT NULL)),
    CHECK ((status = 'complete') = (completed_at IS NOT NULL)),
    CHECK (status = 'complete' OR snapshot_payload IS NOT NULL),
    CHECK (snapshot_payload IS NULL OR octet_length(snapshot_payload::text) <= 1048576)
);
CREATE INDEX IF NOT EXISTS global_radar_campaign_handoffs_pending_idx
    ON global_radar_campaign_handoffs(next_attempt_at,created_at,source_ref) WHERE status='pending';
CREATE INDEX IF NOT EXISTS global_radar_campaign_handoffs_lease_idx
    ON global_radar_campaign_handoffs(lease_until,source_ref) WHERE status='publishing';
CREATE INDEX IF NOT EXISTS global_radar_campaign_handoffs_failed_idx
    ON global_radar_campaign_handoffs(created_at,source_ref) WHERE status='failed';
