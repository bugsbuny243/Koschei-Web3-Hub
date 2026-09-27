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
