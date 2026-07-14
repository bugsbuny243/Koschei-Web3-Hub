package services

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultActorDefenseCorrelationInterval = 10 * time.Minute

// ActorDefenseCorrelator continuously turns a bounded 30-day production sensor
// window into durable wallet-level threat tracks. It performs no Solana RPC
// calls and therefore remains safe for the full Pump discovery volume.
// Expensive live transaction verification stays selective and on-demand.
type ActorDefenseCorrelator struct {
	DB        *sql.DB
	PollEvery time.Duration
}

func NewActorDefenseCorrelator(db *sql.DB) *ActorDefenseCorrelator {
	return &ActorDefenseCorrelator{DB: db, PollEvery: actorDefenseCorrelationInterval()}
}

func (w *ActorDefenseCorrelator) Start(ctx context.Context) {
	if w == nil || w.DB == nil {
		return
	}
	if w.PollEvery <= 0 {
		w.PollEvery = defaultActorDefenseCorrelationInterval
	}
	timer := time.NewTimer(12 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			stats, err := w.RunOnce(runCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Printf("actor defense correlation cycle failed: %v", err)
			} else if stats.CreatorTracks > 0 || stats.HolderTracks > 0 {
				log.Printf("actor defense correlation cycle: changed_creator_tracks=%d changed_repeat_holder_tracks=%d window_days=30", stats.CreatorTracks, stats.HolderTracks)
			}
			timer.Reset(w.PollEvery)
		}
	}
}

type ActorDefenseCorrelationStats struct {
	CreatorTracks int64 `json:"creator_tracks"`
	HolderTracks  int64 `json:"repeat_holder_tracks"`
}

func (w *ActorDefenseCorrelator) RunOnce(ctx context.Context) (ActorDefenseCorrelationStats, error) {
	if w == nil || w.DB == nil {
		return ActorDefenseCorrelationStats{}, nil
	}
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return ActorDefenseCorrelationStats{}, err
	}
	defer tx.Rollback()

	creatorResult, err := tx.ExecContext(ctx, actorDefenseCreatorCorrelationSQL)
	if err != nil {
		return ActorDefenseCorrelationStats{}, err
	}
	holderResult, err := tx.ExecContext(ctx, actorDefenseRepeatHolderCorrelationSQL)
	if err != nil {
		return ActorDefenseCorrelationStats{}, err
	}
	if err := tx.Commit(); err != nil {
		return ActorDefenseCorrelationStats{}, err
	}
	creatorRows, _ := creatorResult.RowsAffected()
	holderRows, _ := holderResult.RowsAffected()
	return ActorDefenseCorrelationStats{CreatorTracks: creatorRows, HolderTracks: holderRows}, nil
}

func actorDefenseCorrelationInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ACTOR_DEFENSE_CORRELATION_SECONDS"))
	if raw == "" {
		return defaultActorDefenseCorrelationInterval
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 120 || seconds > 3600 {
		return defaultActorDefenseCorrelationInterval
	}
	return time.Duration(seconds) * time.Second
}

const actorDefenseCreatorCorrelationSQL = `
WITH creator_tokens AS (
	SELECT
		COALESCE(NULLIF(e.network,''),'solana-mainnet') AS network,
		COALESCE(
			NULLIF(btrim(e.signals->>'creator_wallet'),''),
			NULLIF(btrim(e.signals->>'deployer_wallet'),'')
		) AS wallet,
		e.target,
		min(e.created_at) AS first_seen_at,
		max(e.created_at) AS last_seen_at
	FROM security_radar_events e
	WHERE e.target_type='token'
	  AND e.created_at >= now()-interval '30 days'
	  AND btrim(COALESCE(e.target,''))<>''
	  AND COALESCE(
			NULLIF(btrim(e.signals->>'creator_wallet'),''),
			NULLIF(btrim(e.signals->>'deployer_wallet'),'')
	  ) IS NOT NULL
	GROUP BY COALESCE(NULLIF(e.network,''),'solana-mainnet'),
		COALESCE(NULLIF(btrim(e.signals->>'creator_wallet'),''),NULLIF(btrim(e.signals->>'deployer_wallet'),'')),
		e.target
), creator_rollup AS (
	SELECT network,wallet,count(*)::integer AS token_count,
		min(first_seen_at) AS first_seen_at,max(last_seen_at) AS last_seen_at
	FROM creator_tokens
	GROUP BY network,wallet
	HAVING count(*) >= 2
), latest_holders AS (
	SELECT DISTINCT ON (network,target,owner_wallet)
		network,target,owner_wallet,percentage,scanned_at
	FROM security_radar_holder_snapshots
	WHERE scanned_at >= now()-interval '30 days'
	  AND btrim(COALESCE(owner_wallet,''))<>''
	ORDER BY network,target,owner_wallet,scanned_at DESC,id DESC
), repeated_related AS (
	SELECT ct.network,ct.wallet,lh.owner_wallet,count(DISTINCT ct.target)::integer AS shared_tokens
	FROM creator_tokens ct
	JOIN latest_holders lh ON lh.network=ct.network AND lh.target=ct.target
	WHERE lh.owner_wallet<>ct.wallet AND lh.percentage>=20
	GROUP BY ct.network,ct.wallet,lh.owner_wallet
	HAVING count(DISTINCT ct.target) >= 2
), related_rollup AS (
	SELECT network,wallet,count(*)::integer AS related_actor_count,
		max(shared_tokens)::integer AS max_shared_tokens
	FROM repeated_related
	GROUP BY network,wallet
)
INSERT INTO security_threat_tracks (
	network,target_kind,target_id,state,created_token_count,dominant_holder_token_count,
	traded_token_count,related_actor_count,verified_evidence_count,observed_evidence_count,
	dossier,first_seen_at,last_seen_at,last_investigated_at,created_at,updated_at
)
SELECT
	c.network,'wallet',c.wallet,
	CASE WHEN COALESCE(r.related_actor_count,0)>0 THEN 'correlated' ELSE 'tracked' END,
	c.token_count,0,0,COALESCE(r.related_actor_count,0),0,0,
	jsonb_build_object(
		'auto_correlated',true,
		'actor_role','creator_deployer',
		'observation_window_days',30,
		'created_token_count',c.token_count,
		'repeated_related_actor_count',COALESCE(r.related_actor_count,0),
		'max_shared_token_count',COALESCE(r.max_shared_tokens,0),
		'correlation_scope','Koschei Pump discovery and owner-resolved holder memory; not identity or intent proof'
	),
	c.first_seen_at,c.last_seen_at,c.first_seen_at,now(),now()
FROM creator_rollup c
LEFT JOIN related_rollup r ON r.network=c.network AND r.wallet=c.wallet
ON CONFLICT (network,target_kind,target_id)
DO UPDATE SET
	state=CASE
		WHEN security_threat_tracks.state='alerted' THEN 'alerted'
		WHEN security_threat_tracks.state='verified' THEN 'verified'
		WHEN EXCLUDED.state='correlated' THEN 'correlated'
		WHEN security_threat_tracks.state='correlated' THEN 'correlated'
		ELSE EXCLUDED.state
	END,
	created_token_count=GREATEST(security_threat_tracks.created_token_count,EXCLUDED.created_token_count),
	related_actor_count=GREATEST(security_threat_tracks.related_actor_count,EXCLUDED.related_actor_count),
	dossier=security_threat_tracks.dossier || EXCLUDED.dossier,
	first_seen_at=LEAST(security_threat_tracks.first_seen_at,EXCLUDED.first_seen_at),
	last_seen_at=GREATEST(security_threat_tracks.last_seen_at,EXCLUDED.last_seen_at),
	updated_at=now()
WHERE
	(EXCLUDED.state='correlated' AND security_threat_tracks.state NOT IN ('correlated','verified','alerted')) OR
	security_threat_tracks.created_token_count < EXCLUDED.created_token_count OR
	security_threat_tracks.related_actor_count < EXCLUDED.related_actor_count OR
	security_threat_tracks.first_seen_at > EXCLUDED.first_seen_at OR
	security_threat_tracks.last_seen_at < EXCLUDED.last_seen_at OR
	NOT security_threat_tracks.dossier @> EXCLUDED.dossier`

const actorDefenseRepeatHolderCorrelationSQL = `
WITH latest AS (
	SELECT DISTINCT ON (network,target,owner_wallet)
		network,target,owner_wallet,percentage,scanned_at
	FROM security_radar_holder_snapshots
	WHERE scanned_at >= now()-interval '30 days'
	  AND btrim(COALESCE(owner_wallet,''))<>''
	ORDER BY network,target,owner_wallet,scanned_at DESC,id DESC
), repeat_holders AS (
	SELECT network,owner_wallet,count(DISTINCT target)::integer AS token_count,
		max(percentage) AS max_percentage,min(scanned_at) AS first_seen_at,max(scanned_at) AS last_seen_at
	FROM latest
	WHERE percentage>=20
	GROUP BY network,owner_wallet
	HAVING count(DISTINCT target) >= 2
)
INSERT INTO security_threat_tracks (
	network,target_kind,target_id,state,created_token_count,dominant_holder_token_count,
	traded_token_count,related_actor_count,verified_evidence_count,observed_evidence_count,
	dossier,first_seen_at,last_seen_at,last_investigated_at,created_at,updated_at
)
SELECT
	network,'wallet',owner_wallet,'correlated',0,token_count,0,0,0,0,
	jsonb_build_object(
		'auto_correlated',true,
		'actor_role','repeat_dominant_holder',
		'observation_window_days',30,
		'dominant_holder_token_count',token_count,
		'max_holder_percentage',max_percentage,
		'correlation_scope','Owner-resolved top-five holder snapshots at or above 20 percent; not identity or intent proof'
	),
	first_seen_at,last_seen_at,first_seen_at,now(),now()
FROM repeat_holders
ON CONFLICT (network,target_kind,target_id)
DO UPDATE SET
	state=CASE
		WHEN security_threat_tracks.state='alerted' THEN 'alerted'
		WHEN security_threat_tracks.state='verified' THEN 'verified'
		ELSE 'correlated'
	END,
	dominant_holder_token_count=GREATEST(security_threat_tracks.dominant_holder_token_count,EXCLUDED.dominant_holder_token_count),
	dossier=security_threat_tracks.dossier || EXCLUDED.dossier,
	first_seen_at=LEAST(security_threat_tracks.first_seen_at,EXCLUDED.first_seen_at),
	last_seen_at=GREATEST(security_threat_tracks.last_seen_at,EXCLUDED.last_seen_at),
	updated_at=now()
WHERE
	security_threat_tracks.state NOT IN ('correlated','verified','alerted') OR
	security_threat_tracks.dominant_holder_token_count < EXCLUDED.dominant_holder_token_count OR
	security_threat_tracks.first_seen_at > EXCLUDED.first_seen_at OR
	security_threat_tracks.last_seen_at < EXCLUDED.last_seen_at OR
	NOT security_threat_tracks.dossier @> EXCLUDED.dossier`
