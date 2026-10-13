-- Additive migration: no existing stream data is deleted or rewritten.
-- Apply before deploying the stream writer that populates this column.
ALTER TABLE IF EXISTS security_radar_stream_events
    ADD COLUMN IF NOT EXISTS raw_event_sha256 text;

-- Existing schema declares raw_event NOT NULL; new writes intentionally use NULL.
ALTER TABLE IF EXISTS security_radar_stream_events
    ALTER COLUMN raw_event DROP NOT NULL;

COMMENT ON COLUMN security_radar_stream_events.raw_event_sha256 IS
    'SHA-256 hex digest of the original WSS notification; raw_event is NULL for new writes.';
