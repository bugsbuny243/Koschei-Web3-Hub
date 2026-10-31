-- Owner-only bounded SOC signature readiness reads the newest 20 immutable
-- signed verdict receipts across targets. The existing target-scoped index
-- cannot efficiently serve global recorded_at DESC, id DESC.
CREATE INDEX IF NOT EXISTS idx_unified_verdict_receipts_recent_proof
    ON security_unified_verdict_receipts (recorded_at DESC, id DESC);
