CREATE UNIQUE INDEX IF NOT EXISTS security_radar_verdicts_stream_event_module_uniq
ON security_radar_verdicts ((signals->>'source_stream_event_id'), module_id)
WHERE source='arvis_stream'
  AND COALESCE(signals->>'source_stream_event_id','') <> '';
