-- Koschei Web3 ClickHouse migration 004
--
-- Add read-path skipping indexes without changing ReplacingMergeTree identity.
-- ARVIS memory reads filter verdict snapshots by target/network/source_updated_at
-- and stream events by network/target/event_date/created_at.
--
-- Do not MATERIALIZE here: existing installations can schedule that operation
-- separately after measuring mutation cost. Fresh installations receive these
-- indexes before data ingestion.

ALTER TABLE koschei_web3.arvis_verdict_snapshots
    ADD INDEX IF NOT EXISTS idx_arvis_verdict_target target TYPE bloom_filter(0.01) GRANULARITY 1;

ALTER TABLE koschei_web3.arvis_verdict_snapshots
    ADD INDEX IF NOT EXISTS idx_arvis_verdict_network network TYPE set(64) GRANULARITY 1;

ALTER TABLE koschei_web3.arvis_verdict_snapshots
    ADD INDEX IF NOT EXISTS idx_arvis_verdict_source_updated_at source_updated_at TYPE minmax GRANULARITY 1;

ALTER TABLE koschei_web3.security_radar_stream_events
    ADD INDEX IF NOT EXISTS idx_arvis_stream_target target TYPE bloom_filter(0.01) GRANULARITY 1;

ALTER TABLE koschei_web3.security_radar_stream_events
    ADD INDEX IF NOT EXISTS idx_arvis_stream_created_at created_at TYPE minmax GRANULARITY 1;
