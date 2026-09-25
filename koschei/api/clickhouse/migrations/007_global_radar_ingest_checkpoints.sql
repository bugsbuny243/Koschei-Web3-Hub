-- Koschei Web3 ClickHouse migration 007
--
-- Durable Global Radar ingest checkpoint ledger v1.
--
-- Checkpoints advance only after canonical event persistence succeeds. Exact
-- replay of the same checkpoint converges through ReplacingMergeTree while
-- distinct block hashes at the same height remain visible as historical
-- lineage evidence. This table is operational cursor state, never a risk grade.

CREATE DATABASE IF NOT EXISTS koschei_web3;

CREATE TABLE IF NOT EXISTS koschei_web3.global_radar_ingest_checkpoints
(
    cursor_key String,
    schema_version LowCardinality(String),
    network_id LowCardinality(String),
    stream_kind LowCardinality(String),

    height UInt64,
    block_hash String,
    parent_hash String,
    source_event_sha256 FixedString(64),
    state LowCardinality(String),

    observed_at DateTime64(3, 'UTC'),
    checkpoint_version UInt64,
    recorded_at DateTime64(3, 'UTC') DEFAULT now64(3),

    CONSTRAINT global_radar_ingest_checkpoint_schema_v1
        CHECK schema_version = 'koschei.global-radar-ingest-checkpoint.v1',
    CONSTRAINT global_radar_ingest_checkpoint_cursor_present
        CHECK length(cursor_key) > 0,
    CONSTRAINT global_radar_ingest_checkpoint_block_hash_present
        CHECK length(block_hash) > 0,
    CONSTRAINT global_radar_ingest_checkpoint_event_sha256_hex
        CHECK match(toString(source_event_sha256), '^[0-9a-f]{64}$'),
    CONSTRAINT global_radar_ingest_checkpoint_state
        CHECK state IN ('canonical', 'reorg_observed', 'rewind')
)
ENGINE = ReplacingMergeTree(checkpoint_version)
ORDER BY (cursor_key, height, block_hash, source_event_sha256, state)
SETTINGS index_granularity = 8192;
