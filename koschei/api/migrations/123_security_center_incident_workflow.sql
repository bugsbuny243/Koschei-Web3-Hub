-- Koschei Global Security Center: operator incident workflow v1.
-- This workflow links operator response state to existing evidence references.
-- It never mutates ARVIS verdicts, evidence rows, dossier bundles, or alert records.

CREATE TABLE IF NOT EXISTS security_incident_cases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_ref text NOT NULL UNIQUE DEFAULT (
        'KSI-' || upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 16))
    ),
    title text NOT NULL,
    severity text NOT NULL DEFAULT 'medium',
    status text NOT NULL DEFAULT 'open',
    network text NOT NULL DEFAULT '',
    target text NOT NULL DEFAULT '',
    summary text NOT NULL DEFAULT '',
    evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    alert_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    dossier_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by text NOT NULL DEFAULT 'owner',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    CONSTRAINT security_incident_cases_title_nonempty CHECK (btrim(title) <> ''),
    CONSTRAINT security_incident_cases_severity CHECK (severity IN ('info','low','medium','high','critical')),
    CONSTRAINT security_incident_cases_status CHECK (status IN ('open','investigating','contained','resolved','closed')),
    CONSTRAINT security_incident_cases_evidence_refs_array CHECK (jsonb_typeof(evidence_refs) = 'array'),
    CONSTRAINT security_incident_cases_alert_refs_array CHECK (jsonb_typeof(alert_refs) = 'array'),
    CONSTRAINT security_incident_cases_dossier_refs_array CHECK (jsonb_typeof(dossier_refs) = 'array')
);

CREATE INDEX IF NOT EXISTS security_incident_cases_status_updated_idx
    ON security_incident_cases (status, updated_at DESC);

CREATE INDEX IF NOT EXISTS security_incident_cases_severity_updated_idx
    ON security_incident_cases (severity, updated_at DESC);

CREATE INDEX IF NOT EXISTS security_incident_cases_target_idx
    ON security_incident_cases (network, target)
    WHERE btrim(target) <> '';

CREATE TABLE IF NOT EXISTS security_incident_actions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id uuid NOT NULL REFERENCES security_incident_cases(id) ON DELETE CASCADE,
    action_type text NOT NULL,
    actor text NOT NULL DEFAULT 'owner',
    summary text NOT NULL DEFAULT '',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT security_incident_actions_type CHECK (
        action_type IN (
            'created','note','investigate','contain','resolve','reopen','close',
            'severity_change','link_evidence','link_alert','link_dossier'
        )
    ),
    CONSTRAINT security_incident_actions_payload_object CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS security_incident_actions_incident_created_idx
    ON security_incident_actions (incident_id, created_at DESC);

COMMENT ON TABLE security_incident_cases IS
'Owner-only incident workflow state. Links existing evidence without changing deterministic verdict authority.';
COMMENT ON TABLE security_incident_actions IS
'Append-only operator action timeline for security incidents; actions do not rewrite source evidence or verdicts.';
