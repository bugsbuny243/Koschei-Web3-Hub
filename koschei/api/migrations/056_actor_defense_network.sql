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
    CONSTRAINT security_actor_evidence_amounts_check
        CHECK (amount_native >= 0 AND token_amount >= 0 AND occurrence_count >= 1),
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
    CONSTRAINT security_threat_tracks_counts_check
        CHECK (
            created_token_count >= 0 AND
            dominant_holder_token_count >= 0 AND
            traded_token_count >= 0 AND
            related_actor_count >= 0 AND
            verified_evidence_count >= 0 AND
            observed_evidence_count >= 0
        ),
    CONSTRAINT security_threat_tracks_time_check
        CHECK (first_seen_at <= last_seen_at),
    CONSTRAINT security_threat_tracks_target_unique
        UNIQUE (network,target_kind,target_id)
);

-- Threat memory is monotonic even when separate workers race. A later partial
-- dossier may enrich a track but may not erase stronger state, counters or the
-- earliest/latest observation window already persisted.
CREATE OR REPLACE FUNCTION preserve_security_threat_track_history()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    old_rank integer;
    new_rank integer;
BEGIN
    old_rank := CASE OLD.state
        WHEN 'detected' THEN 1
        WHEN 'tracked' THEN 2
        WHEN 'correlated' THEN 3
        WHEN 'verified' THEN 4
        WHEN 'alerted' THEN 5
        ELSE 0
    END;
    new_rank := CASE NEW.state
        WHEN 'detected' THEN 1
        WHEN 'tracked' THEN 2
        WHEN 'correlated' THEN 3
        WHEN 'verified' THEN 4
        WHEN 'alerted' THEN 5
        ELSE 0
    END;
    IF old_rank > new_rank THEN
        NEW.state := OLD.state;
    END IF;
    NEW.created_token_count := GREATEST(OLD.created_token_count,NEW.created_token_count);
    NEW.dominant_holder_token_count := GREATEST(OLD.dominant_holder_token_count,NEW.dominant_holder_token_count);
    NEW.traded_token_count := GREATEST(OLD.traded_token_count,NEW.traded_token_count);
    NEW.related_actor_count := GREATEST(OLD.related_actor_count,NEW.related_actor_count);
    NEW.verified_evidence_count := GREATEST(OLD.verified_evidence_count,NEW.verified_evidence_count);
    NEW.observed_evidence_count := GREATEST(OLD.observed_evidence_count,NEW.observed_evidence_count);
    NEW.dossier := COALESCE(OLD.dossier,'{}'::jsonb) || COALESCE(NEW.dossier,'{}'::jsonb);
    NEW.first_seen_at := LEAST(OLD.first_seen_at,NEW.first_seen_at);
    NEW.last_seen_at := GREATEST(OLD.last_seen_at,NEW.last_seen_at);
    NEW.last_investigated_at := GREATEST(OLD.last_investigated_at,NEW.last_investigated_at);
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS security_threat_tracks_preserve_history ON security_threat_tracks;
CREATE TRIGGER security_threat_tracks_preserve_history
BEFORE UPDATE ON security_threat_tracks
FOR EACH ROW
EXECUTE FUNCTION preserve_security_threat_track_history();

CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_state_time
    ON security_threat_tracks (state,last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_target
    ON security_threat_tracks (network,target_kind,target_id);
CREATE INDEX IF NOT EXISTS idx_security_threat_tracks_last_investigated
    ON security_threat_tracks (last_investigated_at DESC);

-- The automatic correlation worker reads only a bounded 30-day sensor window.
-- These indexes keep that path independent from total historical table growth.
CREATE INDEX IF NOT EXISTS idx_security_radar_events_creator_observation
    ON security_radar_events (
        network,
        (COALESCE(
            NULLIF(btrim(signals->>'creator_wallet'),''),
            NULLIF(btrim(signals->>'deployer_wallet'),'')
        )),
        target,
        created_at DESC
    )
    WHERE target_type='token'
      AND COALESCE(
            NULLIF(btrim(signals->>'creator_wallet'),''),
            NULLIF(btrim(signals->>'deployer_wallet'),'')
          ) IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_security_radar_holder_snapshots_latest_owner
    ON security_radar_holder_snapshots (
        network,target,owner_wallet,scanned_at DESC,id DESC
    )
    WHERE btrim(owner_wallet) <> '';
