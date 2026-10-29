-- Additive SOC metadata: existing incidents, actions and evidence remain intact.
CREATE TABLE IF NOT EXISTS security_incident_soc (
    incident_id uuid PRIMARY KEY REFERENCES security_incident_cases(id),
    tier text NOT NULL DEFAULT 'L1' CHECK (tier IN ('L1','L2','L3')),
    assignee text NOT NULL DEFAULT '' CHECK (length(assignee) <= 120),
    acknowledged_at timestamptz,
    version bigint NOT NULL CHECK (version > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS security_incident_soc_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    incident_id uuid NOT NULL REFERENCES security_incident_cases(id),
    version bigint NOT NULL,
    action text NOT NULL CHECK (action IN ('assign','escalate','acknowledge')),
    actor text NOT NULL,
    summary text NOT NULL CHECK (length(summary) BETWEEN 1 AND 4000),
    before_state jsonb NOT NULL CHECK (jsonb_typeof(before_state) = 'object'),
    after_state jsonb NOT NULL CHECK (jsonb_typeof(after_state) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (incident_id, version)
);

CREATE INDEX IF NOT EXISTS security_incident_soc_queue_idx
    ON security_incident_soc (tier, assignee);

-- Enforce append-only history independently of application behavior.
CREATE OR REPLACE FUNCTION reject_soc_event_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'SOC events are append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER security_incident_soc_events_immutable
    BEFORE UPDATE OR DELETE ON security_incident_soc_events
    FOR EACH ROW EXECUTE FUNCTION reject_soc_event_mutation();

COMMENT ON TABLE security_incident_soc IS
'Owner-managed analyst queue metadata. Tier and assignee are workflow labels, not authentication roles or enforcement authority.';
