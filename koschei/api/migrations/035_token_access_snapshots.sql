CREATE TABLE IF NOT EXISTS token_access_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject text NOT NULL,
    wallet_address text NOT NULL,
    network text NOT NULL,
    mint_address text NOT NULL,
    amount_raw numeric(78,0) NOT NULL DEFAULT 0,
    decimals integer NOT NULL DEFAULT 0 CHECK (decimals >= 0 AND decimals <= 18),
    tier text NOT NULL DEFAULT 'none' CHECK (tier IN ('none','basic','pro','enterprise')),
    gate_enabled boolean NOT NULL DEFAULT false,
    rpc_source text NOT NULL DEFAULT 'solana_rpc',
    checked_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_token_access_snapshots_subject_checked
    ON token_access_snapshots (auth_subject, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_token_access_snapshots_wallet_mint
    ON token_access_snapshots (network, wallet_address, mint_address, checked_at DESC);
