-- Koschei Web3 ClickHouse migration 005
--
-- Global Radar graph memory v1.
--
-- Append-first intelligence ledger. Application code writes immutable contract
-- records; repeated delivery of the same stable record may converge through
-- ReplacingMergeTree(ingest_version). This is not a transactional uniqueness claim.
--
-- Full contract payload bytes are stored as String so payload_sha256 verifies the
-- exact bytes that were accepted by the writer. Query dimensions are duplicated
-- explicitly so readers never need to trust unparsed payload JSON for boundaries.
--
-- No PARTITION BY or skipping indexes are introduced before representative Global
-- Radar read patterns are measured.

CREATE DATABASE IF NOT EXISTS koschei_web3;

CREATE TABLE IF NOT EXISTS koschei_web3.global_radar_graph_records
(
    record_type LowCardinality(String),
    record_id String,
    schema_version LowCardinality(String),

    snapshot_sha256 FixedString(64),

    network LowCardinality(String),
    secondary_network LowCardinality(String) DEFAULT '',
    subject_id String,
    target_subject_id String DEFAULT '',
    record_kind LowCardinality(String),
    evidence_status LowCardinality(String),
    evidence_refs Array(String),

    observed_at Nullable(DateTime64(9, 'UTC')),
    recorded_at DateTime64(9, 'UTC'),

    payload_json String,
    payload_sha256 FixedString(64),

    ingest_version UInt64,
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    record_date Date MATERIALIZED toDate(recorded_at),

    CONSTRAINT global_radar_record_type_allowed
        CHECK record_type IN ('observation', 'relation', 'bridge_link', 'verdict_reference'),
    CONSTRAINT global_radar_record_id_present
        CHECK length(record_id) > 0,
    CONSTRAINT global_radar_subject_id_present
        CHECK length(subject_id) > 0,
    CONSTRAINT global_radar_snapshot_sha256_hex
        CHECK match(toString(snapshot_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT global_radar_payload_sha256_hex
        CHECK match(toString(payload_sha256), '^[0-9a-f]{64}$')
)
ENGINE = ReplacingMergeTree(ingest_version)
ORDER BY (record_type, network, record_date, subject_id, record_id)
SETTINGS index_granularity = 8192;
