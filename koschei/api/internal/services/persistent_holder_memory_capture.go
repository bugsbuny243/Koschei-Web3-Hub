package services

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const persistentDominantHolderThreshold = 20.0

// CapturePersistentDominantHolderMemory writes owner-resolved top-5 dominance
// directly into retention-independent actor memory. It does not depend on the
// raw holder-snapshot retention path and records only OBSERVED ownership facts.
func CapturePersistentDominantHolderMemory(ctx context.Context, db *sql.DB, network, mint string, holder HolderIntelligence, observedAt time.Time) (int, error) {
	if db == nil || !holder.Available {
		return 0, nil
	}
	network = normalizeRadarNetwork(network)
	mint = strings.TrimSpace(mint)
	if mint == "" {
		return 0, nil
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	persisted := 0
	for _, row := range holder.Rows {
		owner := strings.TrimSpace(row.OwnerWallet)
		if owner == "" || !row.OwnerResolved || !row.RiskBearing || row.ExcludedFromHolderRisk || row.Rank < 1 || row.Rank > 5 {
			continue
		}
		share := row.CirculatingPercentage
		if share <= 0 {
			share = row.RawPercentage
		}
		if share < persistentDominantHolderThreshold {
			continue
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO security_actor_evidence (
				network,actor_wallet,actor_role,counterpart_kind,counterpart_id,relation,
				verification_status,evidence_key,source,observed_at,first_observed_at,last_observed_at,
				program,token_mint,occurrence_count,metadata,created_at,updated_at
			) VALUES (
				$1,$2,'dominant_holder','token',$3,'dominant_holder_of',
				'observed','holder:'||$3,'holder_intelligence_direct',$4,$4,$4,
				'spl-token',$3,1,
				jsonb_build_object(
					'actor_role','dominant_holder',
					'holder_percentage',$5,
					'max_holder_percentage',$5,
					'holder_rank',$6,
					'best_holder_rank',$6,
					'owner_resolved',true,
					'persistent_actor_index',true,
					'capture_path','direct_holder_intelligence'
				),$4,now()
			)
			ON CONFLICT (network,actor_wallet,counterpart_kind,counterpart_id,relation,source,evidence_key)
			DO UPDATE SET
				first_observed_at=LEAST(security_actor_evidence.first_observed_at,EXCLUDED.first_observed_at),
				last_observed_at=GREATEST(security_actor_evidence.last_observed_at,EXCLUDED.last_observed_at),
				observed_at=GREATEST(security_actor_evidence.observed_at,EXCLUDED.observed_at),
				occurrence_count=security_actor_evidence.occurrence_count+1,
				metadata=security_actor_evidence.metadata || jsonb_build_object(
					'actor_role','dominant_holder',
					'holder_percentage',$5,
					'max_holder_percentage',GREATEST(
						CASE WHEN COALESCE(security_actor_evidence.metadata->>'max_holder_percentage','') ~ '^[0-9]+([.][0-9]+)?$'
							THEN (security_actor_evidence.metadata->>'max_holder_percentage')::double precision ELSE 0 END,
						$5
					),
					'holder_rank',$6,
					'best_holder_rank',LEAST(
						CASE WHEN COALESCE(security_actor_evidence.metadata->>'best_holder_rank','') ~ '^[0-9]+$'
							THEN (security_actor_evidence.metadata->>'best_holder_rank')::integer ELSE $6 END,
						$6
					),
					'owner_resolved',true,
					'persistent_actor_index',true,
					'capture_path','direct_holder_intelligence'
				),
				updated_at=now()
		`, network, owner, mint, observedAt.UTC(), share, row.Rank)
		if err != nil {
			return persisted, err
		}
		persisted++
	}
	return persisted, nil
}
