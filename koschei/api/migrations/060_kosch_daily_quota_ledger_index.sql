-- KOSCH daily scan quotas reuse credit_events as the atomic reservation/refund
-- ledger. This partial index keeps UTC-day usage reads bounded without adding a
-- parallel counter table.
CREATE INDEX IF NOT EXISTS idx_credit_events_kosch_daily_quota
    ON credit_events (lower(email), created_at, reason)
    WHERE event_type IN ('kosch_quota_reserve', 'kosch_quota_refund');
