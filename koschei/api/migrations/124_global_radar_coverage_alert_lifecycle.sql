-- Durable lifecycle for Global Radar operational coverage alerts.
-- Episodes record monitoring-state transitions only. They do not claim
-- historical chain-data loss and do not mutate ARVIS evidence or verdicts.

CREATE TABLE IF NOT EXISTS global_radar_coverage_episodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    component_id text NOT NULL,
    network_id text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open',
    coverage_status text NOT NULL,
    coverage_reason text NOT NULL,
    open_alert_id uuid REFERENCES security_alert_events(id) ON DELETE SET NULL,
    recovery_alert_id uuid REFERENCES security_alert_events(id) ON DELETE SET NULL,
    opened_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    recovered_at timestamptz,
    CONSTRAINT global_radar_coverage_episodes_component_nonempty CHECK (btrim(component_id) <> ''),
    CONSTRAINT global_radar_coverage_episodes_status CHECK (status IN ('open','recovered')),
    CONSTRAINT global_radar_coverage_episodes_coverage_status CHECK (
        coverage_status IN ('lagging','blind_spot','reorg_guard','current')
    ),
    CONSTRAINT global_radar_coverage_episodes_recovery_shape CHECK (
        (status='open' AND coverage_status IN ('lagging','blind_spot','reorg_guard') AND recovered_at IS NULL)
        OR
        (status='recovered' AND coverage_status='current' AND recovered_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS global_radar_coverage_episodes_one_open_idx
    ON global_radar_coverage_episodes (component_id)
    WHERE status='open';

CREATE INDEX IF NOT EXISTS global_radar_coverage_episodes_network_opened_idx
    ON global_radar_coverage_episodes (network_id, opened_at DESC);

CREATE INDEX IF NOT EXISTS global_radar_coverage_episodes_status_updated_idx
    ON global_radar_coverage_episodes (status, updated_at DESC);

COMMENT ON TABLE global_radar_coverage_episodes IS
'Durable operational coverage episodes derived from runtime freshness and head-ingest cursor progress; blind_spot is not proof of historical chain-data loss.';
