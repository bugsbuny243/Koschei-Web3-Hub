-- Durable Global Campaign v1 memory.
--
-- This keeps only compact canonical campaign revisions and the current projection
-- in PostgreSQL. Raw/heavy radar event history remains outside this store.

CREATE TABLE IF NOT EXISTS global_campaign_revisions (
    campaign_ref text NOT NULL,
    revision bigint NOT NULL,
    schema_version text NOT NULL,
    state text NOT NULL,
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    ruleset_version text NOT NULL DEFAULT '',
    evidence_hash_sha256 text NOT NULL,
    canonical_payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_ref, revision),
    CONSTRAINT global_campaign_revisions_ref_nonempty CHECK (btrim(campaign_ref) <> ''),
    CONSTRAINT global_campaign_revisions_revision_positive CHECK (revision > 0),
    CONSTRAINT global_campaign_revisions_schema CHECK (schema_version = 'koschei.global-campaign.v1'),
    CONSTRAINT global_campaign_revisions_state CHECK (
        state IN ('emerging','active','escalating','degrading','dormant','closed','reopened')
    ),
    CONSTRAINT global_campaign_revisions_time_order CHECK (last_observed_at >= first_observed_at),
    CONSTRAINT global_campaign_revisions_hash CHECK (evidence_hash_sha256 ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT global_campaign_revisions_payload_object CHECK (jsonb_typeof(canonical_payload) = 'object')
);

CREATE INDEX IF NOT EXISTS global_campaign_revisions_state_last_seen_idx
    ON global_campaign_revisions (state, last_observed_at DESC);

CREATE TABLE IF NOT EXISTS global_campaign_current (
    campaign_ref text PRIMARY KEY,
    revision bigint NOT NULL,
    schema_version text NOT NULL,
    state text NOT NULL,
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    ruleset_version text NOT NULL DEFAULT '',
    evidence_hash_sha256 text NOT NULL,
    canonical_payload jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT global_campaign_current_ref_nonempty CHECK (btrim(campaign_ref) <> ''),
    CONSTRAINT global_campaign_current_revision_positive CHECK (revision > 0),
    CONSTRAINT global_campaign_current_schema CHECK (schema_version = 'koschei.global-campaign.v1'),
    CONSTRAINT global_campaign_current_state CHECK (
        state IN ('emerging','active','escalating','degrading','dormant','closed','reopened')
    ),
    CONSTRAINT global_campaign_current_time_order CHECK (last_observed_at >= first_observed_at),
    CONSTRAINT global_campaign_current_hash CHECK (evidence_hash_sha256 ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT global_campaign_current_payload_object CHECK (jsonb_typeof(canonical_payload) = 'object'),
    CONSTRAINT global_campaign_current_revision_fk
        FOREIGN KEY (campaign_ref, revision)
        REFERENCES global_campaign_revisions (campaign_ref, revision)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS global_campaign_current_state_last_seen_idx
    ON global_campaign_current (state, last_observed_at DESC);
