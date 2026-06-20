CREATE TABLE IF NOT EXISTS arvis_stream_processing (
    stream_event_id uuid PRIMARY KEY REFERENCES security_radar_stream_events(id) ON DELETE CASCADE,
    target text NOT NULL DEFAULT '',
    signature text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS arvis_stream_processing_status_idx
    ON arvis_stream_processing (status, updated_at DESC);

CREATE INDEX IF NOT EXISTS arvis_stream_processing_target_idx
    ON arvis_stream_processing (target);
