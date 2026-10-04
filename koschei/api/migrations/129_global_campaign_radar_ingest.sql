CREATE TABLE IF NOT EXISTS global_campaign_radar_ingest (
    verdict_id uuid PRIMARY KEY REFERENCES security_radar_verdicts(id) ON DELETE CASCADE,
    status text NOT NULL,
    campaign_ref text,
    observation_ref text,
    projection_fingerprint text,
    skip_reason text,
    processed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT global_campaign_radar_ingest_status CHECK (status IN ('projected','skipped')),
    CONSTRAINT global_campaign_radar_ingest_projected_fields CHECK (
        (status='projected' AND btrim(COALESCE(campaign_ref,''))<>'' AND btrim(COALESCE(observation_ref,''))<>'' AND projection_fingerprint ~ '^sha256:[0-9a-f]{64}$' AND skip_reason IS NULL)
        OR
        (status='skipped' AND campaign_ref IS NULL AND observation_ref IS NULL AND projection_fingerprint IS NULL AND btrim(COALESCE(skip_reason,''))<>'')
    )
);

CREATE INDEX IF NOT EXISTS idx_global_campaign_radar_ingest_status_processed
    ON global_campaign_radar_ingest (status, processed_at DESC);

COMMENT ON TABLE global_campaign_radar_ingest IS
    'Durable handoff ledger from signed evidence-backed Security Radar verdicts into canonical Global Radar observations and Global Campaign correlation context.';
