-- Koschei Web3 dedicated entitlement-store provisioning contract.
--
-- This script is intentionally NOT part of application persistence. Run it only
-- against the dedicated database referenced by ENTITLEMENT_DATABASE_URL.
-- It creates the smallest commercial trust plane required for authenticated
-- customer binding, Polar billing, Professional entitlement authorization and
-- atomic output reservation/refund.
--
-- Do not point ENTITLEMENT_DATABASE_URL at DATABASE_URL as an implicit fallback.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS app_user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject TEXT UNIQUE,
    email TEXT NOT NULL,
    plan_id TEXT NOT NULL DEFAULT 'free',
    credits INTEGER NOT NULL DEFAULT 0,
    wallet_address TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    banned_at TIMESTAMPTZ,
    ban_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entitlement_profiles_email_unique
    ON app_user_profiles (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_entitlement_profiles_subject_unique
    ON app_user_profiles (auth_subject)
    WHERE auth_subject IS NOT NULL AND trim(auth_subject) <> '';
CREATE INDEX IF NOT EXISTS idx_entitlement_profiles_status
    ON app_user_profiles (status);

CREATE TABLE IF NOT EXISTS entitlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id TEXT,
    email TEXT,
    plan_id TEXT,
    payment_request_id UUID,
    payment_provider TEXT,
    external_payment_id TEXT,
    outputs_total INTEGER NOT NULL DEFAULT 0,
    outputs_remaining INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    starts_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ,
    order_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_entitlement_store_email_status
    ON entitlements (lower(email), status, expires_at);
CREATE INDEX IF NOT EXISTS idx_entitlement_store_plan_window
    ON entitlements (lower(email), plan_id, status, starts_at, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_entitlement_store_polar_external_payment
    ON entitlements (external_payment_id)
    WHERE payment_provider = 'polar'
      AND external_payment_id IS NOT NULL
      AND external_payment_id <> '';

CREATE TABLE IF NOT EXISTS credit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT,
    amount INTEGER NOT NULL,
    reason TEXT,
    event_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_entitlement_credit_events_email_created
    ON credit_events (lower(email), created_at DESC);

CREATE TABLE IF NOT EXISTS billing_provider_events (
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    external_subscription_id TEXT,
    plan_id TEXT,
    auth_subject TEXT,
    email TEXT,
    product_id TEXT,
    raw_sha256 TEXT NOT NULL CHECK (raw_sha256 ~ '^sha256:[0-9a-f]{64}$'),
    occurred_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, event_id),
    CONSTRAINT billing_provider_events_provider_check CHECK (provider <> ''),
    CONSTRAINT billing_provider_events_plan_check
        CHECK (plan_id IS NULL OR plan_id IN ('starter','professional','enterprise'))
);

CREATE INDEX IF NOT EXISTS idx_entitlement_billing_subscription
    ON billing_provider_events (provider, external_subscription_id, processed_at DESC)
    WHERE external_subscription_id IS NOT NULL;

COMMENT ON TABLE billing_provider_events IS
    'Provider-neutral idempotency/audit ledger. Raw provider payloads are not stored.';

-- KOSC payment channel quote/settlement ledger.
-- Holding a token never grants access. Only a later verified settlement may
-- activate the canonical Professional entitlement.

CREATE TABLE IF NOT EXISTS kosc_payment_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    wallet_address TEXT NOT NULL,
    mint TEXT NOT NULL,
    treasury TEXT NOT NULL,
    usd_amount NUMERIC(20,2) NOT NULL,
    price_usd NUMERIC(38,18) NOT NULL,
    token_decimals INTEGER NOT NULL CHECK (token_decimals >= 0 AND token_decimals <= 18),
    raw_amount NUMERIC(78,0) NOT NULL CHECK (raw_amount > 0),
    price_block_id BIGINT NOT NULL CHECK (price_block_id > 0),
    access_days INTEGER NOT NULL CHECK (access_days > 0),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','settled','expired','cancelled')),
    expires_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kosc_quotes_subject_status
    ON kosc_payment_quotes (auth_subject, status, expires_at DESC);

CREATE TABLE IF NOT EXISTS kosc_payment_settlements (
    signature TEXT PRIMARY KEY,
    quote_id UUID NOT NULL UNIQUE REFERENCES kosc_payment_quotes(id),
    auth_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    wallet_address TEXT NOT NULL,
    mint TEXT NOT NULL,
    treasury TEXT NOT NULL,
    raw_amount NUMERIC(78,0) NOT NULL CHECK (raw_amount > 0),
    slot BIGINT NOT NULL CHECK (slot > 0),
    block_time TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kosc_settlements_subject_created
    ON kosc_payment_settlements (auth_subject, created_at DESC);

COMMIT;
