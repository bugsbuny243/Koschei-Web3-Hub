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

// ActorDefenseCorrelator turns the persistent actor evidence/index into durable
// wallet-level threat tracks. Raw radar tables may be retained for a bounded
// period, but actor memory is all-time and is never rebuilt from a 30-day window.
// The worker performs no Solana RPC calls; expensive verification remains
// selective and on-demand.
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
				log.Printf("actor defense correlation cycle: changed_creator_tracks=%d changed_repeat_holder_tracks=%d memory=persistent_actor_index", stats.CreatorTracks, stats.HolderTracks)
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
		network,
		actor_wallet AS wallet,
		token_mint AS target,
		min(first_observed_at) AS first_seen_at,
		max(last_observed_at) AS last_seen_at,
		bool_or(verification_status='verified') AS verified_relation
	FROM security_actor_evidence
	WHERE actor_role='creator_deployer'
	  AND relation='created_token'
	  AND counterpart_kind='token'
	  AND verification_status IN ('verified','observed')
	  AND token_mint IS NOT NULL
	  AND btrim(token_mint)<>''
	GROUP BY network,actor_wallet,token_mint
), creator_rollup AS (
	SELECT
		network,
		wallet,
		count(*)::integer AS token_count,
		count(*) FILTER (WHERE verified_relation)::integer AS verified_token_count,
		min(first_seen_at) AS first_seen_at,
		max(last_seen_at) AS last_seen_at
	FROM creator_tokens
	GROUP BY network,wallet
	HAVING count(*) >= 2
), holder_tokens AS (
	SELECT
		network,
		actor_wallet AS owner_wallet,
		token_mint AS target,
		max(
			CASE
				WHEN COALESCE(metadata->>'max_holder_percentage',metadata->>'holder_percentage','')
					~ '^[0-9]+([.][0-9]+)?$'
				THEN COALESCE(metadata->>'max_holder_percentage',metadata->>'holder_percentage')::numeric
				ELSE 0
			END
		)::double precision AS max_percentage,
		min(first_observed_at) AS first_seen_at,
		max(last_observed_at) AS last_seen_at
	FROM security_actor_evidence
	WHERE actor_role='dominant_holder'
	  AND relation='dominant_holder_of'
	  AND verification_status IN ('verified','observed')
	  AND token_mint IS NOT NULL
	  AND btrim(token_mint)<>''
	GROUP BY network,actor_wallet,token_mint
), repeated_related AS (
	SELECT
		ct.network,
		ct.wallet,
		ht.owner_wallet,
		count(DISTINCT ct.target)::integer AS shared_tokens,
		max(ht.max_percentage)::double precision AS max_percentage,
		min(ht.first_seen_at) AS first_seen_at,
		max(ht.last_seen_at) AS last_seen_at
	FROM creator_tokens ct
	JOIN holder_tokens ht ON ht.network=ct.network AND ht.target=ct.target
	WHERE ht.owner_wallet<>ct.wallet AND ht.max_percentage>=20
	GROUP BY ct.network,ct.wallet,ht.owner_wallet
	HAVING count(DISTINCT ct.target) >= 2
), related_rollup AS (
	SELECT
		network,
		wallet,
		count(*)::integer AS related_actor_count,
		max(shared_tokens)::integer AS max_shared_tokens,
		max(max_percentage)::double precision AS max_holder_percentage
	FROM repeated_related
	GROUP BY network,wallet
)
INSERT INTO security_threat_tracks (
	network,target_kind,target_id,state,created_token_count,dominant_holder_token_count,
	traded_token_count,related_actor_count,verified_evidence_count,observed_evidence_count,
	dossier,first_seen_at,last_seen_at,last_investigated_at,created_at,updated_at
)
SELECT
	c.network,
	'wallet',
	c.wallet,
	CASE WHEN COALESCE(r.related_actor_count,0)>0 THEN 'correlated' ELSE 'tracked' END,
	c.token_count,
	0,
	0,
	COALESCE(r.related_actor_count,0),
	0,
	0,
	jsonb_build_object(
		'auto_correlated',true,
		'actor_role','creator_deployer',
		'memory_scope','persistent_actor_index',
		'created_token_count',c.token_count,
		'verified_creator_token_count',c.verified_token_count,
		'creator_reuse_evidence_status',CASE WHEN c.verified_token_count>=2 THEN 'verified' ELSE 'observed' END,
		'repeated_related_actor_count',COALESCE(r.related_actor_count,0),
		'max_shared_token_count',COALESCE(r.max_shared_tokens,0),
		'max_related_holder_percentage',COALESCE(r.max_holder_percentage,0),
		'related_actor_evidence_status','observed',
		'correlation_scope','Persistent creator/deployer and owner-resolved holder evidence; not identity or intent proof'
	),
	c.first_seen_at,
	c.last_seen_at,
	now(),
	now(),
	now()
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
WITH holder_tokens AS (
	SELECT
		network,
		actor_wallet AS owner_wallet,
		token_mint AS target,
		max(
			CASE
				WHEN COALESCE(metadata->>'max_holder_percentage',metadata->>'holder_percentage','')
					~ '^[0-9]+([.][0-9]+)?$'
				THEN COALESCE(metadata->>'max_holder_percentage',metadata->>'holder_percentage')::numeric
				ELSE 0
			END
		)::double precision AS max_percentage,
		min(first_observed_at) AS first_seen_at,
		max(last_observed_at) AS last_seen_at
	FROM security_actor_evidence
	WHERE actor_role='dominant_holder'
	  AND relation='dominant_holder_of'
	  AND verification_status IN ('verified','observed')
	  AND token_mint IS NOT NULL
	  AND btrim(token_mint)<>''
	GROUP BY network,actor_wallet,token_mint
), repeat_holders AS (
	SELECT
		network,
		owner_wallet,
		count(DISTINCT target)::integer AS token_count,
		max(max_percentage)::double precision AS max_percentage,
		min(first_seen_at) AS first_seen_at,
		max(last_seen_at) AS last_seen_at
	FROM holder_tokens
	WHERE max_percentage>=20
	GROUP BY network,owner_wallet
	HAVING count(DISTINCT target) >= 2
)
INSERT INTO security_threat_tracks (
	network,target_kind,target_id,state,created_token_count,dominant_holder_token_count,
	traded_token_count,related_actor_count,verified_evidence_count,observed_evidence_count,
	dossier,first_seen_at,last_seen_at,last_investigated_at,created_at,updated_at
)
SELECT
	network,
	'wallet',
	owner_wallet,
	'correlated',
	0,
	token_count,
	0,
	0,
	0,
	0,
	jsonb_build_object(
		'auto_correlated',true,
		'actor_role','repeat_dominant_holder',
		'memory_scope','persistent_actor_index',
		'dominant_holder_token_count',token_count,
		'max_holder_percentage',max_percentage,
		'holder_reuse_evidence_status','observed',
		'correlation_scope','Persistent owner-resolved dominant-holder observations at or above 20 percent; not identity or intent proof'
	),
	first_seen_at,
	last_seen_at,
	now(),
	now(),
	now()
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
