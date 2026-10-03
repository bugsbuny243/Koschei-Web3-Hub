-- Global Campaign -> Security Incident durable linkage.
--
-- Links pin the exact campaign revision and evidence hash observed by the
-- operator workflow. They never mutate campaign evidence, ARVIS verdicts,
-- incident status, or containment state.

CREATE TABLE IF NOT EXISTS security_incident_campaign_links (
    incident_id uuid NOT NULL REFERENCES security_incident_cases(id) ON DELETE CASCADE,
    campaign_ref text NOT NULL,
    campaign_revision bigint NOT NULL,
    evidence_hash_sha256 text NOT NULL,
    linked_by text NOT NULL DEFAULT 'owner',
    linked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, campaign_ref, campaign_revision),
    CONSTRAINT security_incident_campaign_links_campaign_revision_fk
        FOREIGN KEY (campaign_ref, campaign_revision)
        REFERENCES global_campaign_revisions (campaign_ref, revision)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    CONSTRAINT security_incident_campaign_links_ref_nonempty CHECK (btrim(campaign_ref) <> ''),
    CONSTRAINT security_incident_campaign_links_revision_positive CHECK (campaign_revision > 0),
    CONSTRAINT security_incident_campaign_links_hash CHECK (evidence_hash_sha256 ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT security_incident_campaign_links_actor_nonempty CHECK (btrim(linked_by) <> '')
);

CREATE INDEX IF NOT EXISTS security_incident_campaign_links_campaign_idx
    ON security_incident_campaign_links (campaign_ref, campaign_revision DESC, linked_at DESC);

CREATE INDEX IF NOT EXISTS security_incident_campaign_links_incident_idx
    ON security_incident_campaign_links (incident_id, linked_at DESC);

ALTER TABLE security_incident_actions
    DROP CONSTRAINT IF EXISTS security_incident_actions_type;

ALTER TABLE security_incident_actions
    ADD CONSTRAINT security_incident_actions_type CHECK (
        action_type IN (
            'created','note','investigate','contain','resolve','reopen','close',
            'severity_change','link_evidence','link_alert','link_dossier','link_campaign'
        )
    );

COMMENT ON TABLE security_incident_campaign_links IS
'Append-only links from operator incidents to exact Global Campaign revisions; links do not change campaign/verdict/containment authority.';
