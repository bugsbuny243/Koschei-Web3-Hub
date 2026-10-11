-- Additive migration: no existing stream data is deleted or rewritten.
-- Apply before deploying the stream writer that populates this column.
ALTER TABLE IF EXISTS security_radar_stream_events
    ADD COLUMN IF NOT EXISTS raw_event_sha256 text;

COMMENT ON COLUMN security_radar_stream_events.raw_event_sha256 IS
    'SHA-256 hex digest of the original WSS notification; raw_event is NULL for new writes.';
