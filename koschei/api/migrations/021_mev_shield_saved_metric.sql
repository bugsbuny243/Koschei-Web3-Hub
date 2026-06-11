CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS mev_protection_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_wallet text NOT NULL DEFAULT '',
    tx_signature text NOT NULL DEFAULT '',
    estimated_loss_usd numeric(20, 2) NOT NULL DEFAULT 0,
    mev_saved_usd numeric(20, 2) NOT NULL DEFAULT 0,
    jito_tip_used boolean NOT NULL DEFAULT false,
    risk_score integer NOT NULL DEFAULT 0,
    risk_level text NOT NULL DEFAULT 'DÜŞÜK',
    route text NOT NULL DEFAULT '',
    raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS mev_saved_usd numeric(20, 2) NOT NULL DEFAULT 0;
ALTER TABLE mev_protection_events ADD COLUMN IF NOT EXISTS raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb;
UPDATE mev_protection_events SET mev_saved_usd = estimated_loss_usd WHERE mev_saved_usd = 0 AND estimated_loss_usd > 0;

CREATE INDEX IF NOT EXISTS idx_mev_protection_wallet_created ON mev_protection_events (user_wallet, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mev_protection_signature ON mev_protection_events (tx_signature);
CREATE INDEX IF NOT EXISTS idx_mev_protection_saved_created ON mev_protection_events (mev_saved_usd DESC, created_at DESC);
