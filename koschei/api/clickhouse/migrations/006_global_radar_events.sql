-- Koschei Web3 ClickHouse migration 006
--
-- Chain-independent Global Radar event ledger v1.
--
-- `event_sha256` is the canonical identity produced by
-- koschei.global-radar-event.v1. Exact replay of the same event converges through
-- ReplacingMergeTree(ingest_version); a different canonical event remains a
-- distinct historical record.
--
-- The full event JSON is stored as immutable evidence bytes alongside its own
-- payload SHA-256. Source digests remain queryable without dereferencing external
-- evidence artifacts. This table never creates a risk grade or security verdict.

CREATE DATABASE IF NOT EXISTS koschei_web3;

CREATE TABLE IF NOT EXISTS koschei_web3.global_radar_events
(
    event_sha256 FixedString(64),
    schema_version LowCardinality(String),
    producer String,
    kind LowCardinality(String),
    network_id LowCardinality(String),
    subject_kind LowCardinality(String),
    subject_id String,
    evidence_state LowCardinality(String),
    source_digests Array(String),

    observed_at DateTime64(3, 'UTC'),
    payload_json String,
    payload_sha256 FixedString(64),

    ingest_version UInt64,
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    event_date Date MATERIALIZED toDate(observed_at),

    CONSTRAINT global_radar_event_sha256_hex
        CHECK match(toString(event_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT global_radar_event_payload_sha256_hex
        CHECK match(toString(payload_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT global_radar_event_schema_v1
        CHECK schema_version = 'koschei.global-radar-event.v1',
    CONSTRAINT global_radar_event_subject_present
        CHECK length(subject_id) > 0
)
ENGINE = ReplacingMergeTree(ingest_version)
ORDER BY (network_id, event_date, kind, subject_id, event_sha256)
SETTINGS index_granularity = 8192;
