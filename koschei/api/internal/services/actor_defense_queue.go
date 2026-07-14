package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ActorDefenseQueueItem is an operational verification priority, not a risk
// score or wrongdoing claim. Every point is derived from persisted recurrence,
// evidence coverage and observation recency.
type ActorDefenseQueueItem struct {
	Track                ActorDefenseTrack `json:"track"`
	VerificationPriority int               `json:"verification_priority"`
	PriorityReasons      []string          `json:"priority_reasons"`
	NextAction           string            `json:"next_action"`
	NeedsLiveEvidence    bool              `json:"needs_live_evidence"`
}

type ActorDefenseQueue struct {
	Items       []ActorDefenseQueueItem `json:"items"`
	Counts      map[string]int64        `json:"counts"`
	Total       int64                   `json:"total"`
	Network     string                  `json:"network"`
	StateFilter string                  `json:"state_filter,omitempty"`
	GeneratedAt time.Time               `json:"generated_at"`
	Policy      map[string]any          `json:"policy"`
}

func (s *ActorDefenseStore) ListVerificationQueue(ctx context.Context, network, state string, limit, offset int) (ActorDefenseQueue, error) {
	if s == nil || s.DB == nil {
		return ActorDefenseQueue{}, fmt.Errorf("actor defense database is unavailable")
	}
	network = normalizeRadarNetwork(network)
	state = strings.ToLower(strings.TrimSpace(state))
	if state != "" && actorDefenseStateRank(state) == 0 {
		return ActorDefenseQueue{}, fmt.Errorf("invalid actor defense state")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 || offset > 10000 {
		offset = 0
	}

	counts, total, err := s.actorDefenseQueueCounts(ctx, network)
	if err != nil {
		return ActorDefenseQueue{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id::text,network,target_kind,target_id,state,
		       created_token_count,dominant_holder_token_count,traded_token_count,
		       related_actor_count,verified_evidence_count,observed_evidence_count,
		       dossier,first_seen_at,last_seen_at,last_investigated_at
		FROM security_threat_tracks
		WHERE network=$1 AND ($2='' OR state=$2)
		ORDER BY
			CASE state
				WHEN 'correlated' THEN 0
				WHEN 'tracked' THEN 1
				WHEN 'detected' THEN 2
				WHEN 'verified' THEN 3
				WHEN 'alerted' THEN 4
				ELSE 5
			END,
			(related_actor_count + dominant_holder_token_count + created_token_count) DESC,
			last_seen_at DESC,
			id DESC
		LIMIT $3 OFFSET $4`, network, state, limit, offset)
	if err != nil {
		return ActorDefenseQueue{}, err
	}
	defer rows.Close()

	items := make([]ActorDefenseQueueItem, 0, limit)
	for rows.Next() {
		var track ActorDefenseTrack
		var dossierRaw []byte
		if err := rows.Scan(
			&track.ID, &track.Network, &track.TargetKind, &track.TargetID, &track.State,
			&track.CreatedTokenCount, &track.DominantHolderTokenCount, &track.TradedTokenCount,
			&track.RelatedActorCount, &track.VerifiedEvidenceCount, &track.ObservedEvidenceCount,
			&dossierRaw, &track.FirstSeenAt, &track.LastSeenAt, &track.LastInvestigatedAt,
		); err != nil {
			return ActorDefenseQueue{}, err
		}
		track.Dossier = map[string]any{}
		if len(dossierRaw) > 0 {
			_ = json.Unmarshal(dossierRaw, &track.Dossier)
		}
		priority, reasons, nextAction := ActorDefenseVerificationPriority(track, time.Now().UTC())
		items = append(items, ActorDefenseQueueItem{
			Track: track,
			VerificationPriority: priority,
			PriorityReasons: reasons,
			NextAction: nextAction,
			NeedsLiveEvidence: track.VerifiedEvidenceCount == 0 && actorDefenseStateRank(track.State) >= actorDefenseStateRank("correlated"),
		})
	}
	if err := rows.Err(); err != nil {
		return ActorDefenseQueue{}, err
	}

	// SQL first limits the candidate set by durable state and recurrence. This
	// stable in-memory sort applies the transparent deterministic priority.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			left, right := items[j-1], items[j]
			if left.VerificationPriority > right.VerificationPriority ||
				(left.VerificationPriority == right.VerificationPriority && !left.Track.LastSeenAt.Before(right.Track.LastSeenAt)) {
				break
			}
			items[j-1], items[j] = items[j], items[j-1]
		}
	}

	return ActorDefenseQueue{
		Items: items, Counts: counts, Total: total, Network: network, StateFilter: state,
		GeneratedAt: time.Now().UTC(),
		Policy: map[string]any{
			"score_type": "operational_verification_priority",
			"not_token_or_wallet_risk_score": true,
			"no_identity_or_wrongdoing_claim": true,
			"live_evidence_required_for_verified_state": true,
			"priority_factors": []string{"track_state", "creator_recurrence", "dominant_holder_recurrence", "related_actor_recurrence", "evidence_coverage", "observation_recency"},
		},
	}, nil
}

func (s *ActorDefenseStore) actorDefenseQueueCounts(ctx context.Context, network string) (map[string]int64, int64, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT state,count(*)
		FROM security_threat_tracks
		WHERE network=$1
		GROUP BY state`, network)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	counts := map[string]int64{"detected": 0, "tracked": 0, "correlated": 0, "verified": 0, "alerted": 0}
	var total int64
	for rows.Next() {
		var state string
		var count int64
		if err := rows.Scan(&state, &count); err != nil {
			return nil, 0, err
		}
		counts[state] = count
		total += count
	}
	return counts, total, rows.Err()
}

func ActorDefenseVerificationPriority(track ActorDefenseTrack, now time.Time) (int, []string, string) {
	score := 0
	reasons := []string{}
	state := strings.ToLower(strings.TrimSpace(track.State))
	switch state {
	case "correlated":
		score += 30
		reasons = append(reasons, "cross-token correlation")
	case "tracked":
		score += 14
		reasons = append(reasons, "repeat observation")
	case "detected":
		score += 5
		reasons = append(reasons, "initial observation")
	case "verified":
		score += 12
		reasons = append(reasons, "verified evidence ready for review")
	case "alerted":
		score += 20
		reasons = append(reasons, "owner alert state")
	}

	if track.CreatedTokenCount > 0 {
		points := minActorDefenseInt(track.CreatedTokenCount*6, 18)
		score += points
		reasons = append(reasons, fmt.Sprintf("%d creator/deployer token", track.CreatedTokenCount))
	}
	if track.DominantHolderTokenCount > 0 {
		points := minActorDefenseInt(track.DominantHolderTokenCount*8, 24)
		score += points
		reasons = append(reasons, fmt.Sprintf("%d dominant-holder token", track.DominantHolderTokenCount))
	}
	if track.RelatedActorCount > 0 {
		points := minActorDefenseInt(track.RelatedActorCount*5, 20)
		score += points
		reasons = append(reasons, fmt.Sprintf("%d recurring related actor", track.RelatedActorCount))
	}
	if track.ObservedEvidenceCount > 0 {
		score += minActorDefenseInt(track.ObservedEvidenceCount*3, 9)
		reasons = append(reasons, fmt.Sprintf("%d observed evidence", track.ObservedEvidenceCount))
	}
	if track.VerifiedEvidenceCount > 0 {
		score += minActorDefenseInt(track.VerifiedEvidenceCount*2, 6)
		reasons = append(reasons, fmt.Sprintf("%d verified evidence", track.VerifiedEvidenceCount))
	}
	if !track.LastSeenAt.IsZero() {
		age := now.Sub(track.LastSeenAt)
		switch {
		case age < 0 || age <= 24*time.Hour:
			score += 10
			reasons = append(reasons, "observed within 24h")
		case age <= 7*24*time.Hour:
			score += 5
			reasons = append(reasons, "observed within 7d")
		}
	}
	if score > 100 {
		score = 100
	}

	nextAction := "monitor_sensor_memory"
	switch {
	case track.VerifiedEvidenceCount > 0:
		nextAction = "review_verified_evidence"
	case state == "correlated" && track.CreatedTokenCount >= 2:
		nextAction = "verify_creator_funding_and_transfer_chain"
	case state == "correlated":
		nextAction = "collect_live_transaction_evidence"
	case track.DominantHolderTokenCount >= 2:
		nextAction = "expand_cross_token_holder_network"
	case track.CreatedTokenCount >= 2:
		nextAction = "expand_creator_token_history"
	}
	return score, reasons, nextAction
}

func minActorDefenseInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

var _ = sql.ErrNoRows
