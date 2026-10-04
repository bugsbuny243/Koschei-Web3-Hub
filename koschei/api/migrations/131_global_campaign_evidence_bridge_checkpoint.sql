-- Cursor for projecting already-verified PostgreSQL actor evidence into the
-- canonical Global Radar -> Global Campaign runtime.

CREATE TABLE IF NOT EXISTS global_campaign_evidence_bridge_checkpoint (
    checkpoint_key text PRIMARY KEY,
    last_observed_at timestamptz NOT NULL,
    last_evidence_id text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT global_campaign_evidence_bridge_checkpoint_key_nonempty
        CHECK (btrim(checkpoint_key) <> '')
);

INSERT INTO global_campaign_evidence_bridge_checkpoint (
    checkpoint_key,
    last_observed_at,
    last_evidence_id
) VALUES (
    'security-actor-evidence',
    now() - interval '24 hours',
    ''
)
ON CONFLICT (checkpoint_key) DO NOTHING;
