-- Global Campaign runtime crash reconciliation checkpoint.
--
-- ClickHouse is the durable graph ledger and PostgreSQL is the bounded campaign
-- work queue. They intentionally do not share a distributed transaction. This
-- cursor lets the reconciliation worker replay graph snapshots that were stored
-- in ClickHouse but were not durably enqueued before a process interruption.
--
-- The first deployment looks back 24 hours so snapshots written immediately
-- before this migration are not stranded. Replay is idempotent because the
-- campaign queue uses deterministic source references and ON CONFLICT DO NOTHING.

CREATE TABLE IF NOT EXISTS global_campaign_reconciliation_checkpoint (
    checkpoint_key text PRIMARY KEY,
    last_recorded_at timestamptz NOT NULL,
    last_snapshot_sha256 text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT global_campaign_reconciliation_checkpoint_key_nonempty
        CHECK (btrim(checkpoint_key) <> ''),
    CONSTRAINT global_campaign_reconciliation_checkpoint_hash
        CHECK (last_snapshot_sha256 = '' OR last_snapshot_sha256 ~ '^[0-9a-f]{64}$')
);

INSERT INTO global_campaign_reconciliation_checkpoint (
    checkpoint_key,
    last_recorded_at,
    last_snapshot_sha256
) VALUES (
    'clickhouse-global-radar',
    now() - interval '24 hours',
    ''
)
ON CONFLICT (checkpoint_key) DO NOTHING;
