-- Koschei Web3 ClickHouse migration 008
--
-- Durable Global Radar ingest gap ledger v1.
-- A gap means a sequential height range has not yet been durably replayed after
-- an observed ingest failure. It is not proof that the source chain omitted data.

CREATE DATABASE IF NOT EXISTS koschei_web3;

CREATE TABLE IF NOT EXISTS koschei_web3.global_radar_ingest_gaps
(
    gap_key FixedString(64),
    schema_version LowCardinality(String),
    cursor_key String,
    network_id LowCardinality(String),
    stream_kind LowCardinality(String),

    initial_start_height UInt64,
    next_height UInt64,
    end_height UInt64,
    reason LowCardinality(String),
    state LowCardinality(String),

    detected_at DateTime64(3, 'UTC'),
    updated_at DateTime64(3, 'UTC'),
    resolved_at Nullable(DateTime64(3, 'UTC')),

    gap_version UInt64,
    recorded_at DateTime64(3, 'UTC') DEFAULT now64(3),

    CONSTRAINT global_radar_ingest_gap_schema_v1
        CHECK schema_version = 'koschei.global-radar-ingest-gap.v1',
    CONSTRAINT global_radar_ingest_gap_key_sha256
        CHECK match(toString(gap_key), '^[0-9a-f]{64}$'),
    CONSTRAINT global_radar_ingest_gap_cursor_present
        CHECK length(cursor_key) > 0,
    CONSTRAINT global_radar_ingest_gap_reason_present
        CHECK length(reason) > 0,
    CONSTRAINT global_radar_ingest_gap_state
        CHECK state IN ('open', 'resolved'),
    CONSTRAINT global_radar_ingest_gap_range
        CHECK initial_start_height <= end_height
            AND next_height >= initial_start_height
            AND next_height <= end_height + 1,
    CONSTRAINT global_radar_ingest_gap_resolution_shape
        CHECK (state = 'open' AND next_height <= end_height AND isNull(resolved_at))
           OR (state = 'resolved' AND next_height = end_height + 1 AND isNotNull(resolved_at))
)
ENGINE = ReplacingMergeTree(gap_version)
ORDER BY (cursor_key, gap_key)
SETTINGS index_granularity = 8192;
