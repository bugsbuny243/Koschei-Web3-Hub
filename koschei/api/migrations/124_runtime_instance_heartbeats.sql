-- Koschei runtime topology heartbeat v1.
-- Shared application PostgreSQL is the observation plane; this table does not
-- grant authority to a runtime instance or prove traffic/chain coverage.

CREATE TABLE IF NOT EXISTS runtime_instance_heartbeats (
    instance_id text PRIMARY KEY,
    runtime_role text NOT NULL,
    service_name text NOT NULL DEFAULT '',
    environment_name text NOT NULL DEFAULT '',
    deployment_id text NOT NULL DEFAULT '',
    replica_id text NOT NULL DEFAULT '',
    region text NOT NULL DEFAULT '',
    http_enabled boolean NOT NULL DEFAULT false,
    background_workers_enabled boolean NOT NULL DEFAULT false,
    started_at timestamptz NOT NULL,
    heartbeat_at timestamptz NOT NULL,
    stopped_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT runtime_instance_heartbeats_id_nonempty CHECK (btrim(instance_id) <> ''),
    CONSTRAINT runtime_instance_heartbeats_role_check CHECK (runtime_role IN ('combined','api','worker')),
    CONSTRAINT runtime_instance_heartbeats_capability_check CHECK (
        http_enabled OR background_workers_enabled
    )
);

CREATE INDEX IF NOT EXISTS runtime_instance_heartbeats_role_heartbeat_idx
    ON runtime_instance_heartbeats (runtime_role, heartbeat_at DESC);

CREATE INDEX IF NOT EXISTS runtime_instance_heartbeats_live_idx
    ON runtime_instance_heartbeats (heartbeat_at DESC)
    WHERE stopped_at IS NULL;

COMMENT ON TABLE runtime_instance_heartbeats IS
'Operational runtime-presence heartbeats. Rows prove only that an instance recently wrote to shared PostgreSQL; they do not prove request routing, chain coverage, or worker correctness.';
