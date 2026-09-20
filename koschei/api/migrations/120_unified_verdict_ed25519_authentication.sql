ALTER TABLE IF EXISTS security_unified_radar_verdicts
    ADD COLUMN IF NOT EXISTS digest text,
    ADD COLUMN IF NOT EXISTS signature_algorithm text,
    ADD COLUMN IF NOT EXISTS key_id text,
    ADD COLUMN IF NOT EXISTS payload_hash text;

-- Pre-v1 rows used deterministic SHA-256 fingerprints in the signature column.
-- Preserve that identity as a digest, but do not carry the old "signed=true"
-- meaning into the Ed25519 authentication contract.
UPDATE security_unified_radar_verdicts
SET digest = COALESCE(
        NULLIF(btrim(digest), ''),
        NULLIF(btrim(signature), ''),
        NULLIF(btrim(fingerprint), '')
    ),
    signed = false,
    signature = NULL,
    signature_algorithm = NULL,
    key_id = NULL,
    payload_hash = NULL
WHERE signed = true
  AND (
      signature_algorithm IS NULL OR btrim(signature_algorithm) = '' OR
      key_id IS NULL OR btrim(key_id) = '' OR
      payload_hash IS NULL OR btrim(payload_hash) = ''
  );

ALTER TABLE IF EXISTS security_unified_radar_verdicts
    DROP CONSTRAINT IF EXISTS security_unified_radar_authentication_check;

ALTER TABLE IF EXISTS security_unified_radar_verdicts
    ADD CONSTRAINT security_unified_radar_authentication_check
    CHECK (
        (
            signed = false AND
            signature IS NULL AND
            signature_algorithm IS NULL AND
            key_id IS NULL AND
            payload_hash IS NULL
        )
        OR
        (
            signed = true AND
            signature IS NOT NULL AND btrim(signature) <> '' AND
            signature_algorithm = 'ed25519' AND
            key_id IS NOT NULL AND btrim(key_id) <> '' AND
            payload_hash ~ '^sha256:[0-9a-f]{64}$'
        )
    );

CREATE INDEX IF NOT EXISTS idx_security_unified_radar_key_time
    ON security_unified_radar_verdicts (key_id,last_seen_at DESC)
    WHERE signed = true;
