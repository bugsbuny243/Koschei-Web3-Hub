-- Keep account identity and entitlement state aligned.
-- This migration is intentionally idempotent because Railway may retry migrations.

-- Keep a single canonical profile for each normalized non-empty email.
WITH ranked_profiles AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY lower(btrim(email))
               ORDER BY updated_at DESC NULLS LAST,
                        created_at DESC NULLS LAST,
                        CASE COALESCE(status, 'active')
                            WHEN 'banned' THEN 0
                            WHEN 'removed' THEN 1
                            ELSE 2
                        END,
                        id
           ) AS row_number
    FROM app_user_profiles
    WHERE NULLIF(btrim(email), '') IS NOT NULL
)
DELETE FROM app_user_profiles profile
USING ranked_profiles ranked
WHERE profile.id = ranked.id
  AND ranked.row_number > 1;

CREATE UNIQUE INDEX IF NOT EXISTS app_user_profiles_email_normalized_key
    ON app_user_profiles ((lower(btrim(email))))
    WHERE NULLIF(btrim(email), '') IS NOT NULL;

-- Production free plans never grant paid outputs.
UPDATE entitlements
SET outputs_total = 0,
    outputs_remaining = 0,
    updated_at = now()
WHERE COALESCE(plan_id, 'free') = 'free'
  AND (COALESCE(outputs_total, 0) <> 0 OR COALESCE(outputs_remaining, 0) <> 0);

-- Remove duplicate active free rows while retaining one canonical record.
WITH ranked_free_entitlements AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY lower(btrim(email))
               ORDER BY updated_at DESC NULLS LAST, created_at DESC NULLS LAST, id
           ) AS row_number
    FROM entitlements
    WHERE status = 'active'
      AND COALESCE(plan_id, 'free') = 'free'
      AND NULLIF(btrim(email), '') IS NOT NULL
)
UPDATE entitlements entitlement
SET status = 'cancelled',
    updated_at = now(),
    notes = concat_ws(' | ', NULLIF(entitlement.notes, ''), 'auto-cancelled: duplicate free entitlement')
FROM ranked_free_entitlements ranked
WHERE entitlement.id = ranked.id
  AND ranked.row_number > 1;

CREATE UNIQUE INDEX IF NOT EXISTS entitlements_one_active_free_per_email
    ON entitlements ((lower(btrim(email))))
    WHERE status = 'active'
      AND COALESCE(plan_id, 'free') = 'free'
      AND NULLIF(btrim(email), '') IS NOT NULL;

-- Test access must never survive in production data.
UPDATE entitlements
SET status = 'cancelled',
    updated_at = now(),
    notes = concat_ws(' | ', NULLIF(notes, ''), 'auto-cancelled: production test entitlement')
WHERE status = 'active'
  AND lower(COALESCE(plan_id, '')) = 'test';

-- A paid entitlement is valid only while its application profile is active.
UPDATE entitlements entitlement
SET status = 'cancelled',
    updated_at = now(),
    notes = concat_ws(' | ', NULLIF(entitlement.notes, ''), 'auto-cancelled: profile inactive or missing')
WHERE entitlement.status = 'active'
  AND COALESCE(entitlement.plan_id, 'free') <> 'free'
  AND NOT EXISTS (
      SELECT 1
      FROM app_user_profiles profile
      WHERE lower(btrim(profile.email)) = lower(btrim(entitlement.email))
        AND profile.status = 'active'
  );

CREATE OR REPLACE FUNCTION cancel_entitlements_for_disabled_profile()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IN ('banned', 'removed')
       AND OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE entitlements
        SET status = 'cancelled',
            updated_at = now(),
            notes = concat_ws(' | ', NULLIF(notes, ''), 'auto-cancelled: profile disabled')
        WHERE lower(btrim(email)) = lower(btrim(NEW.email))
          AND status = 'active';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS app_user_profiles_cancel_entitlements ON app_user_profiles;
CREATE TRIGGER app_user_profiles_cancel_entitlements
AFTER UPDATE OF status ON app_user_profiles
FOR EACH ROW
EXECUTE FUNCTION cancel_entitlements_for_disabled_profile();

CREATE OR REPLACE FUNCTION require_active_profile_for_paid_entitlement()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status = 'active'
       AND COALESCE(NEW.plan_id, 'free') <> 'free'
       AND NOT EXISTS (
           SELECT 1
           FROM app_user_profiles profile
           WHERE lower(btrim(profile.email)) = lower(btrim(NEW.email))
             AND profile.status = 'active'
       ) THEN
        RAISE EXCEPTION 'active profile required for paid entitlement'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS entitlements_require_active_profile ON entitlements;
CREATE TRIGGER entitlements_require_active_profile
BEFORE INSERT OR UPDATE OF email, plan_id, status ON entitlements
FOR EACH ROW
EXECUTE FUNCTION require_active_profile_for_paid_entitlement();
