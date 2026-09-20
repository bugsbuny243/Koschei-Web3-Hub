ALTER TABLE IF EXISTS security_radar_verdicts
    ADD COLUMN IF NOT EXISTS evidence_verified boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS digest text,
    ADD COLUMN IF NOT EXISTS signature_algorithm text,
    ADD COLUMN IF NOT EXISTS key_id text,
    ADD COLUMN IF NOT EXISTS payload_hash text;

ALTER TABLE IF EXISTS security_radar_verdicts
    ALTER COLUMN signed SET DEFAULT false;

-- Legacy arm rows used deterministic hashes as signatures. Preserve their
-- identity as digest material, but do not claim cryptographic authentication.
UPDATE security_radar_verdicts
SET evidence_verified =
        lower(COALESCE(signals->>'verified_evidence', 'false')) = 'true'
        OR lower(COALESCE(signals->>'real_onchain_evidence', 'false')) = 'true'
        OR lower(COALESCE(signals->>'real_offchain_evidence', 'false')) = 'true',
    digest = COALESCE(NULLIF(btrim(digest), ''), NULLIF(btrim(signature), '')),
    signed = false,
    signature = NULL,
    signature_algorithm = NULL,
    key_id = NULL,
    payload_hash = NULL
WHERE
    signed = true
    AND (
        signature_algorithm IS NULL OR btrim(signature_algorithm) <> 'ed25519'
        OR key_id IS NULL OR btrim(key_id) = ''
        OR payload_hash IS NULL OR payload_hash !~ '^sha256:[0-9a-f]{64}$'
        OR signature IS NULL OR btrim(signature) !~ '^[A-Za-z0-9_-]{86}$'
    );

UPDATE security_radar_verdicts
SET evidence_verified =
        evidence_verified
        OR lower(COALESCE(signals->>'verified_evidence', 'false')) = 'true'
        OR lower(COALESCE(signals->>'real_onchain_evidence', 'false')) = 'true'
        OR lower(COALESCE(signals->>'real_offchain_evidence', 'false')) = 'true',
    digest = COALESCE(NULLIF(btrim(digest), ''), NULLIF(btrim(signature), '')),
    signature = NULL,
    signature_algorithm = NULL,
    key_id = NULL,
    payload_hash = NULL
WHERE signed = false;

DROP INDEX IF EXISTS security_radar_verdicts_signature_module_idx;

CREATE INDEX IF NOT EXISTS security_radar_verdicts_signature_module_idx
    ON security_radar_verdicts (signature, module_id)
    WHERE signature IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS security_radar_verdicts_digest_module_uniq
    ON security_radar_verdicts (digest, module_id)
    WHERE digest IS NOT NULL
      AND COALESCE(source, '') <> 'arvis_stream';

ALTER TABLE IF EXISTS security_radar_verdicts
    DROP CONSTRAINT IF EXISTS security_radar_arm_authentication_check;

ALTER TABLE IF EXISTS security_radar_verdicts
    ADD CONSTRAINT security_radar_arm_authentication_check
    CHECK (
        (
            signed = false
            AND signature IS NULL
            AND signature_algorithm IS NULL
            AND key_id IS NULL
            AND payload_hash IS NULL
        )
        OR
        (
            signed = true
            AND signature IS NOT NULL
            AND btrim(signature) ~ '^[A-Za-z0-9_-]{86}$'
            AND signature_algorithm = 'ed25519'
            AND key_id IS NOT NULL
            AND btrim(key_id) <> ''
            AND payload_hash ~ '^sha256:[0-9a-f]{64}$'
            AND digest ~ '^koschei-radar:[0-9a-f]{64}$'
        )
    );

CREATE INDEX IF NOT EXISTS security_radar_verdicts_key_created_idx
    ON security_radar_verdicts (key_id, created_at DESC)
    WHERE signed = true;
