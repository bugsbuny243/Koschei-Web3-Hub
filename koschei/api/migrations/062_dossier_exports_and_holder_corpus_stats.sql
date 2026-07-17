CREATE TABLE IF NOT EXISTS dossier_source_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    mint text NOT NULL,
    network text NOT NULL DEFAULT 'solana-mainnet',
    verdict_id text,
    verdict_signature text NOT NULL,
    ruleset_version text NOT NULL,
    produced_at timestamptz NOT NULL,
    source_payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT dossier_source_snapshots_signature_unique UNIQUE (verdict_signature),
    CONSTRAINT dossier_source_snapshots_payload_object CHECK (jsonb_typeof(source_payload) = 'object')
);

CREATE INDEX IF NOT EXISTS dossier_source_snapshots_mint_produced_idx
    ON dossier_source_snapshots (mint, produced_at DESC);

CREATE TABLE IF NOT EXISTS dossier_exports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    case_ref text NOT NULL UNIQUE,
    mint text NOT NULL,
    verdict_id text,
    source_snapshot_id uuid NOT NULL REFERENCES dossier_source_snapshots(id) ON DELETE RESTRICT,
    bundle_hash text NOT NULL,
    canonical_bundle bytea NOT NULL,
    bundle_json jsonb NOT NULL,
    requested_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT dossier_exports_hash_format CHECK (bundle_hash ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT dossier_exports_bundle_object CHECK (jsonb_typeof(bundle_json) = 'object')
);

CREATE INDEX IF NOT EXISTS dossier_exports_mint_created_idx
    ON dossier_exports (mint, created_at DESC);

CREATE TABLE IF NOT EXISTS holder_concentration_corpus_stats (
    stats_key text PRIMARY KEY,
    sample_count bigint NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
    bucket_width numeric(8,4) NOT NULL DEFAULT 1 CHECK (bucket_width > 0),
    bucket_counts jsonb NOT NULL DEFAULT '[]'::jsonb,
    calculated_at timestamptz NOT NULL DEFAULT now(),
    source_window_start timestamptz,
    source_window_end timestamptz,
    CONSTRAINT holder_corpus_bucket_array CHECK (jsonb_typeof(bucket_counts) = 'array')
);

INSERT INTO holder_concentration_corpus_stats
(stats_key, sample_count, bucket_width, bucket_counts, calculated_at)
VALUES ('owner_resolved_top_share_v1', 0, 1, '[]'::jsonb, now())
ON CONFLICT (stats_key) DO NOTHING;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'security_unified_radar_grade_check'
          AND conrelid = 'security_unified_radar_verdicts'::regclass
    ) THEN
        ALTER TABLE security_unified_radar_verdicts
            DROP CONSTRAINT security_unified_radar_grade_check;
    END IF;
    ALTER TABLE security_unified_radar_verdicts
        ADD CONSTRAINT security_unified_radar_grade_check
        CHECK (grade IN ('-','A','B','C','D','E','F'));
EXCEPTION WHEN duplicate_object THEN
    NULL;
END $$;
