-- Public promotion/community evidence for Koschei Web3 / ARVIS.
--
-- This table stores provenance-preserving observations from public sources.
-- It is evidence context only: a public post, handle, URL or repeated claim does
-- not prove common control, real-world identity, market manipulation or crime.

CREATE TABLE IF NOT EXISTS public.public_promotion_evidence (
    observation_ref text PRIMARY KEY,
    schema_version text NOT NULL DEFAULT 'koschei.public-promotion-evidence.v1',
    network text NOT NULL,
    asset_ref text NOT NULL,
    platform text NOT NULL,
    external_id text NOT NULL DEFAULT '',
    canonical_url text NOT NULL,
    canonical_domain text NOT NULL,
    public_actor text NOT NULL DEFAULT '',
    source_ref text NOT NULL,
    evidence_state text NOT NULL,
    claim_fingerprint_sha256 text NOT NULL DEFAULT '',
    content_hash_sha256 text NOT NULL,
    excerpt text NOT NULL DEFAULT '',
    published_at timestamptz,
    observed_at timestamptz NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    verdict_authority boolean NOT NULL DEFAULT false,
    grade_authority boolean NOT NULL DEFAULT false,
    same_operator_claim boolean NOT NULL DEFAULT false,
    real_world_identity_claim boolean NOT NULL DEFAULT false,
    wrongdoing_claim boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT public_promotion_evidence_ref_check CHECK (
        observation_ref ~ '^KPUB1-[0-9A-F]{32}$'
    ),
    CONSTRAINT public_promotion_evidence_identity_check CHECK (
        btrim(network) <> '' AND btrim(asset_ref) <> '' AND btrim(source_ref) <> ''
        AND btrim(canonical_url) <> '' AND btrim(canonical_domain) <> ''
    ),
    CONSTRAINT public_promotion_evidence_platform_check CHECK (
        platform IN ('x','telegram','youtube','reddit','discord','website','other')
    ),
    CONSTRAINT public_promotion_evidence_state_check CHECK (
        evidence_state IN ('verified','observed','unverified')
    ),
    CONSTRAINT public_promotion_evidence_url_check CHECK (
        canonical_url ~ '^https?://'
    ),
    CONSTRAINT public_promotion_evidence_content_hash_check CHECK (
        content_hash_sha256 ~ '^sha256:[0-9a-f]{64}$'
    ),
    CONSTRAINT public_promotion_evidence_claim_hash_check CHECK (
        claim_fingerprint_sha256 = '' OR claim_fingerprint_sha256 ~ '^sha256:[0-9a-f]{64}$'
    ),
    CONSTRAINT public_promotion_evidence_excerpt_check CHECK (
        char_length(excerpt) <= 480
    ),
    CONSTRAINT public_promotion_evidence_metadata_check CHECK (
        jsonb_typeof(metadata) = 'object'
    ),
    CONSTRAINT public_promotion_evidence_no_authority_check CHECK (
        verdict_authority = false
        AND grade_authority = false
        AND same_operator_claim = false
        AND real_world_identity_claim = false
        AND wrongdoing_claim = false
    )
);

CREATE INDEX IF NOT EXISTS idx_public_promotion_evidence_asset_time
    ON public.public_promotion_evidence (network, asset_ref, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_public_promotion_evidence_actor_time
    ON public.public_promotion_evidence (platform, public_actor, observed_at DESC)
    WHERE btrim(public_actor) <> '';

CREATE INDEX IF NOT EXISTS idx_public_promotion_evidence_domain_time
    ON public.public_promotion_evidence (canonical_domain, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_public_promotion_evidence_claim
    ON public.public_promotion_evidence (claim_fingerprint_sha256, observed_at DESC)
    WHERE claim_fingerprint_sha256 <> '';

CREATE OR REPLACE FUNCTION public.reject_public_promotion_evidence_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'public promotion evidence is append-only';
END;
$$;

DROP TRIGGER IF EXISTS public_promotion_evidence_immutable
    ON public.public_promotion_evidence;
CREATE TRIGGER public_promotion_evidence_immutable
BEFORE UPDATE OR DELETE ON public.public_promotion_evidence
FOR EACH ROW EXECUTE FUNCTION public.reject_public_promotion_evidence_mutation();
