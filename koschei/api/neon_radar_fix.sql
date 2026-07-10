-- Koschei Security Radar public-feed hardening
-- Run each CREATE INDEX CONCURRENTLY statement separately in Neon SQL Editor.

-- STEP 1: feed query indexes (safe to rerun)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_srv_module_created
ON public.security_radar_verdicts (module_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_srv_module_target_risk
ON public.security_radar_verdicts (module_id, target, risk_index DESC, created_at DESC);

-- STEP 2: post-deploy feed verification
-- Expected:
--   * cards_per_token = 1 on every row
--   * infrastructure_leak = false on every row
--   * unsigned_or_unverified_leak = false on every row
-- The all-time branch is used only when the verified 24-hour set is empty.
WITH excluded_mints(mint) AS (
  VALUES
    ('So11111111111111111111111111111111111111112'),
    ('EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v'),
    ('Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB'),
    ('JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN'),
    ('4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R'),
    ('DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263'),
    ('7vfCXTUXx5WJV5JADk17DUJ4ksgau7utNKj4b963voxs'),
    ('mSoLzYCxHdYgdzU16g5QSh3i5K3z3KZK7ytfqcJm7So'),
    ('7dHbWXmci3dT8UFYWYZweBLXgycu7Y3iL6trKn1Y7ARj'),
    ('J1toso1uCk3RLmjorhTtrVwY9HJ7X8V9yYac6Y7kGCPn'),
    ('bSo13r4TkiE4KumL71LsHTPpL2euBYLFx6h9HP3piy1'),
    ('HZ1JovNiVvGrGNiiYvEozEVgZ58xaU3RKwX8eACQBCt3'),
    ('orcaEKTdK7LKz57vaAYr9QeNsVEPfiu6QeMU1kektZE'),
    ('jtojtomepa8beP8AuQc6eXt5FriJwfFMwQx2v2f9mCL')
),
recent_cards AS MATERIALIZED (
  SELECT DISTINCT ON (v.target)
    v.id,
    v.target,
    v.risk_index,
    v.risk_level,
    v.signed,
    v.signature,
    v.signals,
    v.created_at,
    '24h'::text AS feed_window
  FROM public.security_radar_verdicts v
  LEFT JOIN excluded_mints x ON x.mint = v.target
  WHERE v.module_id = 'final_verdict_engine'
    AND v.signed = true
    AND v.signature IS NOT NULL
    AND btrim(v.signature) <> ''
    AND btrim(v.target) <> ''
    AND x.mint IS NULL
    AND (
      COALESCE(v.signals->>'verified_evidence', 'false') = 'true'
      OR COALESCE(v.signals->>'real_onchain_evidence', 'false') = 'true'
      OR COALESCE(v.signals->>'real_offchain_evidence', 'false') = 'true'
    )
    AND v.created_at >= now() - interval '24 hours'
  ORDER BY v.target, v.risk_index DESC, v.created_at DESC, v.id DESC
),
fallback_cards AS (
  SELECT DISTINCT ON (v.target)
    v.id,
    v.target,
    v.risk_index,
    v.risk_level,
    v.signed,
    v.signature,
    v.signals,
    v.created_at,
    'all_time_fallback'::text AS feed_window
  FROM public.security_radar_verdicts v
  LEFT JOIN excluded_mints x ON x.mint = v.target
  WHERE NOT EXISTS (SELECT 1 FROM recent_cards)
    AND v.module_id = 'final_verdict_engine'
    AND v.signed = true
    AND v.signature IS NOT NULL
    AND btrim(v.signature) <> ''
    AND btrim(v.target) <> ''
    AND x.mint IS NULL
    AND (
      COALESCE(v.signals->>'verified_evidence', 'false') = 'true'
      OR COALESCE(v.signals->>'real_onchain_evidence', 'false') = 'true'
      OR COALESCE(v.signals->>'real_offchain_evidence', 'false') = 'true'
    )
  ORDER BY v.target, v.risk_index DESC, v.created_at DESC, v.id DESC
),
feed AS (
  SELECT * FROM recent_cards
  UNION ALL
  SELECT * FROM fallback_cards
)
SELECT
  f.target,
  f.risk_index,
  f.risk_level,
  f.feed_window,
  f.created_at,
  count(*) OVER (PARTITION BY f.target) AS cards_per_token,
  EXISTS (SELECT 1 FROM excluded_mints x WHERE x.mint = f.target) AS infrastructure_leak,
  (
    NOT f.signed
    OR f.signature IS NULL
    OR btrim(f.signature) = ''
    OR NOT (
      COALESCE(f.signals->>'verified_evidence', 'false') = 'true'
      OR COALESCE(f.signals->>'real_onchain_evidence', 'false') = 'true'
      OR COALESCE(f.signals->>'real_offchain_evidence', 'false') = 'true'
    )
  ) AS unsigned_or_unverified_leak
FROM feed f
ORDER BY f.risk_index DESC, f.created_at DESC
LIMIT 100;

-- STEP 3: optional 30-day retention preview
-- Review these counts before enabling any DELETE statement.
SELECT 'security_radar_verdicts' AS table_name, count(*) AS rows_older_than_30d
FROM public.security_radar_verdicts
WHERE created_at < now() - interval '30 days'
UNION ALL
SELECT 'security_radar_events', count(*)
FROM public.security_radar_events
WHERE created_at < now() - interval '30 days'
UNION ALL
SELECT 'security_radar_seen_signatures', count(*)
FROM public.security_radar_seen_signatures
WHERE created_at < now() - interval '30 days';

-- Optional cleanup. Uncomment one batch at a time, rerun until it deletes 0 rows,
-- then let Neon autovacuum reclaim dead tuples. Keep this disabled until the
-- preview counts and retention policy are approved.

-- DELETE FROM public.security_radar_verdicts
-- WHERE ctid IN (
--   SELECT ctid
--   FROM public.security_radar_verdicts
--   WHERE created_at < now() - interval '30 days'
--   ORDER BY created_at
--   LIMIT 50000
-- );

-- DELETE FROM public.security_radar_events e
-- WHERE e.ctid IN (
--   SELECT old.ctid
--   FROM public.security_radar_events old
--   WHERE old.created_at < now() - interval '30 days'
--     AND NOT EXISTS (
--       SELECT 1
--       FROM public.security_radar_verdicts v
--       WHERE v.event_id = old.id
--     )
--   ORDER BY old.created_at
--   LIMIT 50000
-- );

-- DELETE FROM public.security_radar_seen_signatures
-- WHERE ctid IN (
--   SELECT ctid
--   FROM public.security_radar_seen_signatures
--   WHERE created_at < now() - interval '30 days'
--   ORDER BY created_at
--   LIMIT 50000
-- );
