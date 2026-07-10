-- KOSCHEI STRUCTURAL RISK FLOOR
-- Run before deploying security_radar_structural.go.
-- Safe to rerun. Backfill accepts only signed, verified observations and
-- ignores zero-valued holder placeholders.

CREATE TABLE IF NOT EXISTS public.token_structural_signals (
  target text NOT NULL,
  network text NOT NULL,
  largest_holder_pct integer NOT NULL DEFAULT 0 CHECK (largest_holder_pct BETWEEN 0 AND 100),
  top10_holder_pct integer NOT NULL DEFAULT 0 CHECK (top10_holder_pct BETWEEN 0 AND 100),
  has_holder_data boolean NOT NULL DEFAULT false,
  mint_authority_present boolean NOT NULL DEFAULT false,
  freeze_authority_present boolean NOT NULL DEFAULT false,
  has_authority_data boolean NOT NULL DEFAULT false,
  holder_observed_at timestamptz,
  authority_observed_at timestamptz,
  observed_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (target, network)
);

ALTER TABLE public.token_structural_signals
  ADD COLUMN IF NOT EXISTS holder_observed_at timestamptz,
  ADD COLUMN IF NOT EXISTS authority_observed_at timestamptz;

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
latest_holder AS (
  SELECT DISTINCT ON (v.target, v.network)
    v.target,
    v.network,
    GREATEST(0, LEAST(100, (v.signals->>'largest_holder_percentage')::numeric::int)) AS largest_holder_pct,
    GREATEST(0, LEAST(100, (v.signals->>'top_10_holder_percentage')::numeric::int)) AS top10_holder_pct,
    v.created_at AS holder_observed_at
  FROM public.security_radar_verdicts v
  LEFT JOIN excluded_mints x ON x.mint = v.target
  WHERE x.mint IS NULL
    AND v.signed = true
    AND btrim(COALESCE(v.signature,'')) <> ''
    AND (v.signals ? 'largest_holder_percentage')
    AND (v.signals ? 'top_10_holder_percentage')
    AND (
      COALESCE(v.signals->>'verified_evidence','false') = 'true'
      OR COALESCE(v.signals->>'real_onchain_evidence','false') = 'true'
      OR COALESCE(v.signals->>'real_offchain_evidence','false') = 'true'
    )
    AND (
      COALESCE(NULLIF(v.signals->>'largest_holder_percentage','')::numeric, 0) > 0
      OR COALESCE(NULLIF(v.signals->>'top_10_holder_percentage','')::numeric, 0) > 0
      OR COALESCE(NULLIF(v.signals->>'largest_accounts','')::numeric, 0) > 0
    )
  ORDER BY v.target, v.network, v.created_at DESC, v.id DESC
),
latest_authority AS (
  SELECT DISTINCT ON (v.target, v.network)
    v.target,
    v.network,
    (v.signals->>'mint_authority_present')::boolean AS mint_authority_present,
    (v.signals->>'freeze_authority_present')::boolean AS freeze_authority_present,
    v.created_at AS authority_observed_at
  FROM public.security_radar_verdicts v
  LEFT JOIN excluded_mints x ON x.mint = v.target
  WHERE x.mint IS NULL
    AND v.signed = true
    AND btrim(COALESCE(v.signature,'')) <> ''
    AND (v.signals ? 'mint_authority_present')
    AND (v.signals ? 'freeze_authority_present')
    AND (
      COALESCE(v.signals->>'verified_evidence','false') = 'true'
      OR COALESCE(v.signals->>'real_onchain_evidence','false') = 'true'
      OR COALESCE(v.signals->>'real_offchain_evidence','false') = 'true'
    )
    AND (
      v.module_id = 'token_authority_scanner'
      OR COALESCE(v.signals->>'is_token_mint','false') = 'true'
    )
  ORDER BY v.target, v.network, v.created_at DESC, v.id DESC
),
keys AS (
  SELECT target, network FROM latest_holder
  UNION
  SELECT target, network FROM latest_authority
),
backfill AS (
  SELECT
    k.target,
    k.network,
    COALESCE(h.largest_holder_pct, 0) AS largest_holder_pct,
    COALESCE(h.top10_holder_pct, 0) AS top10_holder_pct,
    (h.target IS NOT NULL) AS has_holder_data,
    COALESCE(a.mint_authority_present, false) AS mint_authority_present,
    COALESCE(a.freeze_authority_present, false) AS freeze_authority_present,
    (a.target IS NOT NULL) AS has_authority_data,
    h.holder_observed_at,
    a.authority_observed_at,
    GREATEST(
      COALESCE(h.holder_observed_at, '-infinity'::timestamptz),
      COALESCE(a.authority_observed_at, '-infinity'::timestamptz)
    ) AS observed_at
  FROM keys k
  LEFT JOIN latest_holder h USING (target, network)
  LEFT JOIN latest_authority a USING (target, network)
)
INSERT INTO public.token_structural_signals
  (target, network, largest_holder_pct, top10_holder_pct, has_holder_data,
   mint_authority_present, freeze_authority_present, has_authority_data,
   holder_observed_at, authority_observed_at, observed_at, updated_at)
SELECT
  target, network, largest_holder_pct, top10_holder_pct, has_holder_data,
  mint_authority_present, freeze_authority_present, has_authority_data,
  holder_observed_at, authority_observed_at, observed_at, now()
FROM backfill
ON CONFLICT (target, network) DO UPDATE SET
  largest_holder_pct = CASE
    WHEN EXCLUDED.has_holder_data AND COALESCE(EXCLUDED.holder_observed_at, '-infinity'::timestamptz)
         >= COALESCE(token_structural_signals.holder_observed_at, '-infinity'::timestamptz)
      THEN EXCLUDED.largest_holder_pct
    ELSE token_structural_signals.largest_holder_pct
  END,
  top10_holder_pct = CASE
    WHEN EXCLUDED.has_holder_data AND COALESCE(EXCLUDED.holder_observed_at, '-infinity'::timestamptz)
         >= COALESCE(token_structural_signals.holder_observed_at, '-infinity'::timestamptz)
      THEN EXCLUDED.top10_holder_pct
    ELSE token_structural_signals.top10_holder_pct
  END,
  has_holder_data = token_structural_signals.has_holder_data OR EXCLUDED.has_holder_data,
  holder_observed_at = GREATEST(token_structural_signals.holder_observed_at, EXCLUDED.holder_observed_at),
  mint_authority_present = CASE
    WHEN EXCLUDED.has_authority_data AND COALESCE(EXCLUDED.authority_observed_at, '-infinity'::timestamptz)
         >= COALESCE(token_structural_signals.authority_observed_at, '-infinity'::timestamptz)
      THEN EXCLUDED.mint_authority_present
    ELSE token_structural_signals.mint_authority_present
  END,
  freeze_authority_present = CASE
    WHEN EXCLUDED.has_authority_data AND COALESCE(EXCLUDED.authority_observed_at, '-infinity'::timestamptz)
         >= COALESCE(token_structural_signals.authority_observed_at, '-infinity'::timestamptz)
      THEN EXCLUDED.freeze_authority_present
    ELSE token_structural_signals.freeze_authority_present
  END,
  has_authority_data = token_structural_signals.has_authority_data OR EXCLUDED.has_authority_data,
  authority_observed_at = GREATEST(token_structural_signals.authority_observed_at, EXCLUDED.authority_observed_at),
  observed_at = GREATEST(token_structural_signals.observed_at, EXCLUDED.observed_at),
  updated_at = now();

-- ANSEM validation. Production history currently yields largest=88, top10=96.
SELECT *,
       5 + CASE
         WHEN largest_holder_pct >= 60 THEN 28
         WHEN largest_holder_pct >= 35 THEN 20
         WHEN largest_holder_pct >= 20 THEN 10
         ELSE 0
       END + CASE
         WHEN top10_holder_pct >= 90 THEN 22
         WHEN top10_holder_pct >= 75 THEN 14
         WHEN top10_holder_pct >= 55 THEN 8
         ELSE 0
       END AS holder_floor
FROM public.token_structural_signals
WHERE target = '9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump';

SELECT count(*) AS protected_tokens,
       count(*) FILTER (WHERE has_holder_data) AS holder_protected,
       count(*) FILTER (WHERE has_authority_data) AS authority_protected,
       count(*) FILTER (
         WHERE GREATEST(
           COALESCE(holder_observed_at, '-infinity'::timestamptz),
           COALESCE(authority_observed_at, '-infinity'::timestamptz)
         ) >= now() - interval '7 days'
       ) AS fresh_rows
FROM public.token_structural_signals;
