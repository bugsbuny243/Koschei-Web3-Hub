-- Koschei Web3 ClickHouse migration 001
--
-- Additive shadow only. The existing production data path remains authoritative
-- until row parity, replay, failure injection, and evidence-integrity gates pass.
--
-- This table intentionally starts without PARTITION BY. Retention/lifecycle
-- partitioning will be chosen from measured production data rather than guessed
-- before the first ClickHouse workload exists.
--
-- Chain-native identifiers are byte-exact. Never lowercase Solana addresses,
-- signatures, program IDs, or future chain-native identifiers before storage.

CREATE DATABASE IF NOT EXISTS koschei_web3;

CREATE TABLE IF NOT EXISTS koschei_web3.security_radar_stream_events
(
    event_id UUID,
    event_key FixedString(64),

    provider LowCardinality(String),
    stream_mode LowCardinality(String),
    network LowCardinality(String),
    module_id LowCardinality(String),
    event_type LowCardinality(String),

    target String DEFAULT '',
    target_type LowCardinality(String) DEFAULT 'unknown',
    signature String DEFAULT '',
    slot UInt64 DEFAULT 0,
    program_id LowCardinality(String) DEFAULT '',
    evidence_quality LowCardinality(String) DEFAULT 'raw_stream',

    decoded JSON,
    raw_event JSON,
    decoded_sha256 FixedString(64),
    raw_event_sha256 FixedString(64),

    schema_version UInt16 DEFAULT 1,
    created_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    ingest_version UInt64,
    event_date Date MATERIALIZED toDate(created_at),

    CONSTRAINT event_key_sha256_hex CHECK match(toString(event_key), '^[0-9a-f]{64}$'),
    CONSTRAINT decoded_sha256_hex CHECK match(toString(decoded_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT raw_event_sha256_hex CHECK match(toString(raw_event_sha256), '^[0-9a-f]{64}$')
)
ENGINE = ReplacingMergeTree(ingest_version)
ORDER BY (network, module_id, stream_mode, event_date, event_id)
SETTINGS index_granularity = 8192;
