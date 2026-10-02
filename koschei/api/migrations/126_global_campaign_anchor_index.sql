-- Evidence-only lookup index for deterministic Global Campaign convergence.
--
-- The index contains canonical evidence references already present in the
-- compact PostgreSQL campaign projection. It does not duplicate raw radar
-- payloads and it deliberately excludes entity-only hints and verdict refs
-- from automatic campaign matching.

CREATE TABLE IF NOT EXISTS global_campaign_anchor_index (
    campaign_ref text NOT NULL,
    revision bigint NOT NULL,
    anchor_kind text NOT NULL,
    anchor_ref text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (anchor_kind, anchor_ref, campaign_ref),
    CONSTRAINT global_campaign_anchor_index_revision_positive CHECK (revision > 0),
    CONSTRAINT global_campaign_anchor_index_kind CHECK (
        anchor_kind IN (
            'observation',
            'relation',
            'bridge_link',
            'campaign_genome',
            'campaign_tempo',
            'behavior_signature',
            'incident',
            'attack_path'
        )
    ),
    CONSTRAINT global_campaign_anchor_index_ref_nonempty CHECK (btrim(anchor_ref) <> ''),
    CONSTRAINT global_campaign_anchor_index_current_fk
        FOREIGN KEY (campaign_ref)
        REFERENCES global_campaign_current (campaign_ref)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,
    CONSTRAINT global_campaign_anchor_index_revision_fk
        FOREIGN KEY (campaign_ref, revision)
        REFERENCES global_campaign_revisions (campaign_ref, revision)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS global_campaign_anchor_index_campaign_idx
    ON global_campaign_anchor_index (campaign_ref, revision);

-- Backfill any compact current projections written before this migration. The
-- current revision remains the authority boundary for candidate lookup.
INSERT INTO global_campaign_anchor_index (campaign_ref, revision, anchor_kind, anchor_ref)
SELECT
    current_campaign.campaign_ref,
    current_campaign.revision,
    anchor.anchor_kind,
    btrim(anchor.anchor_ref)
FROM global_campaign_current AS current_campaign
CROSS JOIN LATERAL (
    SELECT 'observation'::text AS anchor_kind, value AS anchor_ref
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'observation_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'relation', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'relation_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'bridge_link', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'bridge_link_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'campaign_genome', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'campaign_genome_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'campaign_tempo', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'campaign_tempo_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'behavior_signature', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'behavior_signature_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'incident', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'incident_refs', '[]'::jsonb))
    UNION ALL
    SELECT 'attack_path', value
    FROM jsonb_array_elements_text(COALESCE(current_campaign.canonical_payload->'attack_path_refs', '[]'::jsonb))
) AS anchor
WHERE btrim(anchor.anchor_ref) <> ''
ON CONFLICT (anchor_kind, anchor_ref, campaign_ref)
DO UPDATE SET revision = EXCLUDED.revision;
