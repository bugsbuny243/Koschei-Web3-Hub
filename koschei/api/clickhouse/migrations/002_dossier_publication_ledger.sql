-- Koschei Web3 ClickHouse publication/evidence ledger v1.
-- Append-only by design: current visibility is derived from a verified transition
-- chain; UPDATE/DELETE semantics are intentionally not part of this contract.

CREATE TABLE IF NOT EXISTS koschei_web3.dossier_bundle_manifests
(
    manifest_id UUID,
    case_ref String,
    bundle_sha256 FixedString(64),
    artifact_uri String,
    artifact_sha256 FixedString(64),

    dossier_version LowCardinality(String),
    network LowCardinality(String),
    target_kind LowCardinality(String),
    target_id String,

    verdict_grade LowCardinality(String) DEFAULT '',
    verdict_status LowCardinality(String) DEFAULT '',
    ruleset_version LowCardinality(String) DEFAULT '',

    evidence_rows UInt32 DEFAULT 0,
    verified_rows UInt32 DEFAULT 0,
    observed_rows UInt32 DEFAULT 0,
    inferred_rows UInt32 DEFAULT 0,
    unknown_rows UInt32 DEFAULT 0,

    acceptance_pass UInt32 DEFAULT 0,
    acceptance_fail UInt32 DEFAULT 0,
    acceptance_not_investigated UInt32 DEFAULT 0,

    produced_at DateTime64(3, 'UTC'),
    manifest_sha256 FixedString(64),
    schema_version UInt16 DEFAULT 1,
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),

    CONSTRAINT dossier_manifest_case_ref CHECK match(case_ref, '^KD1-[a-z2-7]{32}$'),
    CONSTRAINT dossier_manifest_bundle_hash CHECK match(toString(bundle_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT dossier_manifest_artifact_hash CHECK match(toString(artifact_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT dossier_manifest_hash CHECK match(toString(manifest_sha256), '^[0-9a-f]{64}$')
)
ENGINE = MergeTree
ORDER BY (case_ref, manifest_id, bundle_sha256)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS koschei_web3.dossier_publication_transitions
(
    transition_id UUID,
    case_ref String,
    sequence UInt64,

    action Enum8(
        'publish' = 1,
        'hide' = 2,
        'draft' = 3,
        'update' = 4,
        'feature' = 5,
        'unfeature' = 6
    ),
    status Enum8(
        'draft' = 1,
        'public' = 2,
        'hidden' = 3
    ),

    actor LowCardinality(String),
    published_by LowCardinality(String),
    public_title String DEFAULT '',
    public_summary String DEFAULT '',
    featured UInt8 DEFAULT 0,
    redaction_profile LowCardinality(String) DEFAULT 'public-onchain-v1',

    manifest_id UUID,
    bundle_sha256 FixedString(64),

    previous_transition_id UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),
    previous_transition_sha256 FixedString(64) DEFAULT repeat('0', 64),
    transition_sha256 FixedString(64),

    occurred_at DateTime64(3, 'UTC'),
    exposure_started_at Nullable(DateTime64(3, 'UTC')),

    schema_version UInt16 DEFAULT 1,
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),

    CONSTRAINT dossier_transition_case_ref CHECK match(case_ref, '^KD1-[a-z2-7]{32}$'),
    CONSTRAINT dossier_transition_bundle_hash CHECK match(toString(bundle_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT dossier_transition_previous_hash CHECK match(toString(previous_transition_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT dossier_transition_hash CHECK match(toString(transition_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT dossier_transition_featured CHECK featured IN (0, 1),
    CONSTRAINT dossier_transition_featured_public CHECK featured = 0 OR status = 'public',
    CONSTRAINT dossier_transition_publisher CHECK published_by IN ('owner', 'koschei-autopublish/v1'),
    CONSTRAINT dossier_transition_actor CHECK actor IN ('owner', 'autopublish'),
    CONSTRAINT dossier_transition_actor_publisher CHECK
        (published_by = 'owner' AND actor = 'owner')
        OR (published_by = 'koschei-autopublish/v1' AND actor = 'autopublish'),
    CONSTRAINT dossier_transition_genesis_link CHECK
        (
            sequence = 1
            AND previous_transition_id = toUUID('00000000-0000-0000-0000-000000000000')
            AND previous_transition_sha256 = repeat('0', 64)
        )
        OR
        (
            sequence > 1
            AND previous_transition_id != toUUID('00000000-0000-0000-0000-000000000000')
            AND previous_transition_sha256 != repeat('0', 64)
        )
)
ENGINE = MergeTree
ORDER BY (case_ref, sequence, transition_id)
SETTINGS index_granularity = 8192;
