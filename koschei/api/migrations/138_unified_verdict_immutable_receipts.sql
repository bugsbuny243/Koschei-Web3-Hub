-- Signed ARVIS final receipts: append-only decision identity, separate from the mutable fingerprint cache.
-- Apply before deploying the writer. This table intentionally has no backfill from mutable historical rows.
CREATE TABLE IF NOT EXISTS security_unified_verdict_receipts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    verdict_id uuid NOT NULL REFERENCES security_unified_radar_verdicts(id),
    network text NOT NULL CHECK (btrim(network) <> ''),
    target_kind text NOT NULL CHECK (btrim(target_kind) <> ''),
    target_id text NOT NULL CHECK (btrim(target_id) <> ''),
    signature text NOT NULL UNIQUE CHECK (signature ~ '^[A-Za-z0-9_-]{86}$'),
    signature_algorithm text NOT NULL CHECK (signature_algorithm = 'ed25519'),
    key_id text NOT NULL CHECK (btrim(key_id) <> ''),
    payload_hash text NOT NULL CHECK (payload_hash ~ '^sha256:[0-9a-f]{64}$'),
    signed_payload bytea NOT NULL CHECK (octet_length(signed_payload) > 0),
    recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_unified_verdict_receipts_target_time
    ON security_unified_verdict_receipts (network,target_kind,target_id,recorded_at DESC);

-- Updates, deletes, and TRUNCATE are rejected at the database layer.
-- Operators with DDL/superuser rights still control the database; this is not external WORM storage.
CREATE OR REPLACE FUNCTION reject_unified_verdict_receipt_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'signed unified verdict receipts are append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER security_unified_verdict_receipts_no_rewrite
    BEFORE UPDATE OR DELETE ON security_unified_verdict_receipts
    FOR EACH ROW EXECUTE FUNCTION reject_unified_verdict_receipt_mutation();

CREATE TRIGGER security_unified_verdict_receipts_no_truncate
    BEFORE TRUNCATE ON security_unified_verdict_receipts
    FOR EACH STATEMENT EXECUTE FUNCTION reject_unified_verdict_receipt_mutation();

COMMENT ON TABLE security_unified_verdict_receipts IS
'Immutable-at-SQL-DML signed ARVIS payload snapshots, not a cryptographic trust root. Verify against independently configured trusted public keys.';
