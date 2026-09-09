CREATE TABLE IF NOT EXISTS koschei_web3.arvis_verdict_snapshots
(
    verdict_id UUID,
    event_id UUID DEFAULT toUUID('00000000-0000-0000-0000-000000000000'),

    module_id LowCardinality(String),
    target String,
    target_type LowCardinality(String) DEFAULT 'unknown',
    network LowCardinality(String),

    grade LowCardinality(String) DEFAULT '',
    risk_index UInt16 DEFAULT 0,
    risk_level LowCardinality(String) DEFAULT '',
    verdict LowCardinality(String),
    recommendation String DEFAULT '',

    evidence Array(String),
    evidence_sha256 FixedString(64),
    signals JSON,
    signals_sha256 FixedString(64),

    rule_version LowCardinality(String),
    signed UInt8,
    signature String DEFAULT '',
    source LowCardinality(String),
    event_type LowCardinality(String) DEFAULT '',
    provider LowCardinality(String) DEFAULT '',

    created_at DateTime64(3, 'UTC'),
    source_updated_at DateTime64(3, 'UTC'),
    mirrored_at DateTime64(3, 'UTC') DEFAULT now64(3),
    source_version UInt64,
    schema_version UInt16 DEFAULT 1,
    verdict_date Date MATERIALIZED toDate(created_at),

    CONSTRAINT arvis_verdict_risk_index_range CHECK risk_index <= 1000,
    CONSTRAINT arvis_verdict_signed_range CHECK signed IN (0, 1),
    CONSTRAINT arvis_verdict_evidence_sha256_hex CHECK match(evidence_sha256, '^[0-9a-f]{64}$'),
    CONSTRAINT arvis_verdict_signals_sha256_hex CHECK match(signals_sha256, '^[0-9a-f]{64}$')
)
ENGINE = ReplacingMergeTree(source_version)
ORDER BY (module_id, verdict_id)
SETTINGS index_granularity = 8192;
