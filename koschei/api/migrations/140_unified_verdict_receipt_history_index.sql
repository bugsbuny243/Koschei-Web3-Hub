-- Owner-only Ed25519 historical receipt browsing by exact network and target.
-- This index allows keyset retrieval of old immutable decisions even when
-- the mutable fingerprint projection has been updated by later scans.
-- It grants no additional read or write privileges.
CREATE INDEX IF NOT EXISTS idx_unified_verdict_receipts_subject_history
    ON security_unified_verdict_receipts
       (network, target_kind, target_id, recorded_at DESC, id DESC);
