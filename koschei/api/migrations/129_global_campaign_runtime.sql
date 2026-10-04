-- Compact, replay-safe handoff from canonical Radar graph storage to Campaign.
-- Heavy/raw evidence remains in ClickHouse. No response authority is granted.
CREATE TABLE IF NOT EXISTS global_campaign_runtime_queue (
    source_ref text PRIMARY KEY CHECK (source_ref ~ '^sha256:[0-9a-f]{64}$'),
    reference_payload jsonb NOT NULL CHECK (jsonb_typeof(reference_payload) = 'object'),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processed','failed')),
    error_code text NOT NULL DEFAULT '',
    campaign_ref text,
    campaign_revision bigint,
    command_payload jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    CHECK (octet_length(reference_payload::text) <= 262144),
    CHECK ((status = 'processed') = (command_payload IS NOT NULL)),
    FOREIGN KEY (campaign_ref, campaign_revision)
        REFERENCES global_campaign_revisions (campaign_ref, revision)
        ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS global_campaign_runtime_pending_idx
    ON global_campaign_runtime_queue (created_at, source_ref) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS global_campaign_runtime_result_idx
    ON global_campaign_runtime_queue (campaign_ref, campaign_revision, processed_at DESC)
    WHERE status = 'processed';
