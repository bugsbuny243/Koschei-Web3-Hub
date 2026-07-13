CREATE TABLE IF NOT EXISTS security_actor_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    network text NOT NULL DEFAULT 'solana-mainnet',
    actor_wallet text NOT NULL,
    counterpart_kind text NOT NULL,
    counterpart_id text NOT NULL,
    relation text NOT NULL,
    verification_status text NOT NULL DEFAULT 'observed',
    evidence_key text NOT NULL,
    source text NOT NULL DEFAULT 'koschei',
    signature text,
    slot bigint,
    observed_at timestamptz NOT NULL DEFAULT now(),
    amount_native numeric NOT NULL DEFAULT 0,
    token_mint text,
    token_amount numeric NOT NULL DEFAULT 0,
    occurrence_count bigint NOT NULL DEFAULT 1,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT security_actor_evidence_status_check
        CHECK (verification_status IN ('verified','observed','inferred','unverified')),
    CONSTRAINT security_actor_evidence_nonempty_check
        CHECK (
            btrim(actor_wallet) <> '' AND
            btrim(counterpart_kind) <> '' AND
            btrim(counterpart_id) <> '' AND
            btrim(relation) <> '' AND
            btrim(evidence_key) <> '' AND
            btrim(source) <> ''
        ),
    CONSTRAINT security_actor_evidence_unique
        UNIQUE (network,actor_wallet,counterpart_kind,counterpart_id,relation,source,evidence_key)
);

CREATE INDEX IF NOT EXISTS idx_security_actor_evidence_actor_time
    ON security_actor_evidence (network,actor_wallet,observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_actor_evidence_counterpart_time
    ON security_actor_evidence (network,counterpart_kind,counterpart_id,observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_actor_evidence_relation_time
    ON security_actor_evidence (network,relation,observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_actor_evidence_signature
    ON security_actor_evidence (signature)
    WHERE signature IS NOT NULL AND btrim(signature) <> '';
CREATE INDEX IF NOT EXISTS idx_security_actor_evidence_token_mint
    ON security_actor_evidence (token_mint,observed_at DESC)
    WHERE token_mint IS NOT NULL AND btrim(token_mint) <> '';

CREATE TABLE IF NOT EXISTS security_threat_tracks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    network text NOT NULL DEFAULT 'solana-mainnet',
    target_kind text NOT NULL DEFAULT 'wallet',
    target_id text NOT NULL,
    state text NOT NULL DEFAULT 'detected',
    created_token_count integer NOT NULL DEFAULT 0,
    dominant_holder_token_count integer NOT NULL DEFAULT 0,
    traded_token_count integer NOT NULL DEFAULT 0,
    related_actor_count integer NOT NULL DEFAULT 0,
    verified_evidence_count integer NOT NULL DEFAULT 0,
    observed_evidence_count integer NOT NULL DEFAULT 0,
    dossier jsonb NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    last_investigated_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT security_threat_tracks_state_check
        CHECK (state IN ('detected','tracked','correlated','verified','alerted')),
    CONSTRAINT security_threat_tracks_nonempty_check
        CHECK (btrim(target_kind) <> '' AND btrim(target_id) <> ''),
    CONSTRAINT security_threat_tracks_target_unique
        UNIQUE (network,target_kind,target_id)
);

CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_state_time
    ON security_threat_tracks (state,last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_target
    ON security_threat_tracks (network,target_kind,target_id);
CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_last_investigated
    ON security_threat_tracks (last_investigated_at DESC);
