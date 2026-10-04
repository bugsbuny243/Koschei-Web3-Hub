package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/runtimehealth"
)

const GlobalCampaignEvidenceBridgeHealthID = "worker.global-campaign-evidence-bridge"
const globalCampaignEvidenceBridgeCheckpointKey = "security-actor-evidence"

type globalCampaignEvidenceCursor struct {
	ObservedAt time.Time
	EvidenceID string
}

type globalCampaignActorEvidenceRow struct {
	ID              string
	Network         string
	ActorWallet     string
	CounterpartKind string
	CounterpartID   string
	Relation        string
	EvidenceKey     string
	Source          string
	Signature       string
	Slot            int64
	ObservedAt      time.Time
	TokenMint       string
	Metadata        map[string]any
}

func VerifyGlobalCampaignEvidenceBridgeSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrGlobalCampaignStoreUnavailable
	}
	var checkpoint, evidence, verdicts bool
	if err := db.QueryRowContext(ctx, `SELECT
        to_regclass('global_campaign_evidence_bridge_checkpoint') IS NOT NULL,
        to_regclass('security_actor_evidence') IS NOT NULL,
        to_regclass('security_radar_verdicts') IS NOT NULL`).Scan(&checkpoint, &evidence, &verdicts); err != nil {
		return err
	}
	if !checkpoint || !evidence || !verdicts {
		return errors.New("campaign evidence bridge migrations are required")
	}
	return nil
}

func loadGlobalCampaignEvidenceCursor(ctx context.Context, db *sql.DB) (globalCampaignEvidenceCursor, error) {
	var cursor globalCampaignEvidenceCursor
	err := db.QueryRowContext(ctx, `SELECT last_observed_at,last_evidence_id
        FROM global_campaign_evidence_bridge_checkpoint WHERE checkpoint_key=$1`, globalCampaignEvidenceBridgeCheckpointKey).
		Scan(&cursor.ObservedAt, &cursor.EvidenceID)
	if errors.Is(err, sql.ErrNoRows) {
		return cursor, errors.New("campaign evidence bridge checkpoint is missing")
	}
	if err != nil {
		return cursor, err
	}
	cursor.ObservedAt = cursor.ObservedAt.UTC()
	cursor.EvidenceID = strings.TrimSpace(cursor.EvidenceID)
	return cursor, nil
}

func advanceGlobalCampaignEvidenceCursor(ctx context.Context, db *sql.DB, cursor globalCampaignEvidenceCursor) error {
	if cursor.ObservedAt.IsZero() || strings.TrimSpace(cursor.EvidenceID) == "" {
		return errors.New("invalid campaign evidence cursor")
	}
	result, err := db.ExecContext(ctx, `UPDATE global_campaign_evidence_bridge_checkpoint
        SET last_observed_at=$2,last_evidence_id=$3,updated_at=clock_timestamp()
        WHERE checkpoint_key=$1
          AND (last_observed_at,last_evidence_id) < ($2,$3)`,
		globalCampaignEvidenceBridgeCheckpointKey, cursor.ObservedAt.UTC(), strings.TrimSpace(cursor.EvidenceID))
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 1 {
		return nil
	}
	current, err := loadGlobalCampaignEvidenceCursor(ctx, db)
	if err != nil {
		return err
	}
	if current.ObservedAt.After(cursor.ObservedAt) ||
		(current.ObservedAt.Equal(cursor.ObservedAt) && current.EvidenceID >= cursor.EvidenceID) {
		return nil
	}
	return errors.New("campaign evidence bridge cursor did not advance")
}

func readGlobalCampaignActorEvidence(ctx context.Context, db *sql.DB, cursor globalCampaignEvidenceCursor, limit int) ([]globalCampaignActorEvidenceRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := db.QueryContext(ctx, `SELECT
        id::text,network,actor_wallet,counterpart_kind,counterpart_id,relation,
        evidence_key,source,COALESCE(signature,''),COALESCE(slot,0),observed_at,
        COALESCE(token_mint,''),metadata
      FROM security_actor_evidence
      WHERE verification_status='verified'
        AND (observed_at,id::text) > ($1,$2)
      ORDER BY observed_at ASC,id::text ASC
      LIMIT $3`, cursor.ObservedAt.UTC(), cursor.EvidenceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]globalCampaignActorEvidenceRow, 0, limit)
	for rows.Next() {
		var item globalCampaignActorEvidenceRow
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.Network, &item.ActorWallet, &item.CounterpartKind, &item.CounterpartID,
			&item.Relation, &item.EvidenceKey, &item.Source, &item.Signature, &item.Slot, &item.ObservedAt,
			&item.TokenMint, &metadataRaw); err != nil {
			return nil, err
		}
		item.ObservedAt = item.ObservedAt.UTC()
		item.Metadata = map[string]any{}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
				return nil, fmt.Errorf("decode verified actor evidence metadata: %w", err)
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func globalCampaignMetadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func globalCampaignEvidenceObservation(kind string, subject IntelligenceSubject, evidence IntelligenceEvidence) (GlobalRadarObservation, error) {
	return BuildGlobalRadarObservation(kind, subject, evidence)
}

func buildGlobalCampaignActorEvidenceSnapshot(ctx context.Context, db *sql.DB, row globalCampaignActorEvidenceRow) (GlobalRadarSnapshot, bool, error) {
	sourceNetwork := strings.ToLower(strings.TrimSpace(row.Network))
	if sourceNetwork == "" {
		sourceNetwork = "solana-mainnet"
	}
	targetNetwork := strings.ToLower(strings.TrimSpace(globalCampaignMetadataString(row.Metadata, "target_network", "counterpart_network", "destination_network")))
	if targetNetwork == "" {
		targetNetwork = sourceNetwork
	}
	sourceSubject := ClassifyIntelligenceSubject(strings.TrimSpace(row.ActorWallet), sourceNetwork)
	targetSubject := ClassifyIntelligenceSubject(strings.TrimSpace(row.CounterpartID), targetNetwork)
	if sourceSubject.ChainFamily == IntelligenceChainFamilyUnknown || targetSubject.ChainFamily == IntelligenceChainFamilyUnknown ||
		strings.TrimSpace(sourceSubject.ID) == "" || strings.TrimSpace(targetSubject.ID) == "" || sourceSubject.ID == targetSubject.ID {
		return GlobalRadarSnapshot{}, false, nil
	}

	relationEvidenceID := "actor-relation:" + strings.TrimSpace(row.ID)
	attributes := map[string]any{
		"source_subject_id":  sourceSubject.ID,
		"target_subject_id":  targetSubject.ID,
		"source_network":     sourceSubject.Network,
		"target_network":     targetSubject.Network,
		"relation":           strings.TrimSpace(row.Relation),
		"counterpart_kind":   strings.TrimSpace(row.CounterpartKind),
		"evidence_key":       strings.TrimSpace(row.EvidenceKey),
		"source":             strings.TrimSpace(row.Source),
		"authority_boundary": "verified_relation_projection_only",
	}
	if strings.TrimSpace(row.TokenMint) != "" {
		attributes["token_mint"] = strings.TrimSpace(row.TokenMint)
	}
	sourceEvidence := IntelligenceEvidence{
		ID:              relationEvidenceID,
		SubjectID:       sourceSubject.ID,
		ChainFamily:     sourceSubject.ChainFamily,
		Chain:           sourceSubject.Chain,
		Network:         sourceSubject.Network,
		Source:          "security_actor_evidence:" + strings.TrimSpace(row.Source),
		Status:          IntelligenceEvidenceVerified,
		TransactionHash: strings.TrimSpace(row.Signature),
		BlockOrSlot:     row.Slot,
		ObservedAt:      row.ObservedAt.UTC(),
		Address:         strings.TrimSpace(row.ActorWallet),
		Confidence:      1,
		Attributes:      attributes,
	}
	targetEvidence := IntelligenceEvidence{
		ID:          "actor-counterpart:" + strings.TrimSpace(row.ID),
		SubjectID:   targetSubject.ID,
		ChainFamily: targetSubject.ChainFamily,
		Chain:       targetSubject.Chain,
		Network:     targetSubject.Network,
		Source:      "security_actor_evidence:" + strings.TrimSpace(row.Source),
		Status:      IntelligenceEvidenceVerified,
		ObservedAt:  row.ObservedAt.UTC(),
		Address:     strings.TrimSpace(row.CounterpartID),
		Confidence:  1,
		Attributes: map[string]any{
			"relation_evidence_ref": relationEvidenceID,
			"counterpart_kind":      strings.TrimSpace(row.CounterpartKind),
			"authority_boundary":    "verified_relation_projection_only",
		},
	}
	sourceObservation, err := globalCampaignEvidenceObservation(GlobalRadarObservationTransaction, sourceSubject, sourceEvidence)
	if err != nil {
		return GlobalRadarSnapshot{}, false, err
	}
	targetObservation, err := globalCampaignEvidenceObservation(GlobalRadarObservationTransaction, targetSubject, targetEvidence)
	if err != nil {
		return GlobalRadarSnapshot{}, false, err
	}
	relation, err := BuildGlobalRadarRelationEdge(sourceSubject, targetSubject, row.Relation, sourceEvidence)
	if err != nil {
		return GlobalRadarSnapshot{}, false, err
	}
	if relation.Status != IntelligenceEvidenceVerified {
		return GlobalRadarSnapshot{}, false, nil
	}
	observations := []GlobalRadarObservation{sourceObservation, targetObservation}
	generatedAt := row.ObservedAt.UTC()
	if threat, threatAt, ok, err := loadGlobalCampaignVerifiedThreatObservation(ctx, db, targetSubject); err != nil {
		return GlobalRadarSnapshot{}, false, err
	} else if ok {
		observations = append(observations, threat)
		if threatAt.After(generatedAt) {
			generatedAt = threatAt
		}
	}
	snapshot, err := BuildGlobalRadarSnapshot(observations, []GlobalRadarRelationEdge{relation}, nil, nil, generatedAt)
	if err != nil {
		return GlobalRadarSnapshot{}, false, err
	}
	return snapshot, true, nil
}

func loadGlobalCampaignVerifiedThreatObservation(ctx context.Context, db *sql.DB, subject IntelligenceSubject) (GlobalRadarObservation, time.Time, bool, error) {
	var riskIndex int
	var riskLevel, verdict, recommendation, ruleVersion, payloadHash, digest string
	var createdAt time.Time
	err := db.QueryRowContext(ctx, `SELECT risk_index,risk_level,verdict,recommendation,rule_version,
        COALESCE(payload_hash,''),COALESCE(digest,''),created_at
      FROM security_radar_verdicts
      WHERE module_id='final_verdict_engine'
        AND signed=true AND evidence_verified=true
        AND network=$1 AND target=$2
      ORDER BY created_at DESC
      LIMIT 1`, subject.Network, subject.Raw).
		Scan(&riskIndex, &riskLevel, &verdict, &recommendation, &ruleVersion, &payloadHash, &digest, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GlobalRadarObservation{}, time.Time{}, false, nil
	}
	if err != nil {
		return GlobalRadarObservation{}, time.Time{}, false, err
	}
	evidenceID := strings.TrimSpace(payloadHash)
	if evidenceID == "" {
		evidenceID = strings.TrimSpace(digest)
	}
	if evidenceID == "" {
		return GlobalRadarObservation{}, time.Time{}, false, nil
	}
	evidence := IntelligenceEvidence{
		ID:          "arvis-final:" + evidenceID,
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      "arvis_final_verdict",
		Status:      IntelligenceEvidenceVerified,
		ObservedAt:  createdAt.UTC(),
		Address:     subject.Raw,
		Confidence:  1,
		Attributes: map[string]any{
			"risk_index":         riskIndex,
			"risk_level":         strings.TrimSpace(riskLevel),
			"verdict":            strings.TrimSpace(verdict),
			"recommendation":     strings.TrimSpace(recommendation),
			"rule_version":       strings.TrimSpace(ruleVersion),
			"payload_hash":       strings.TrimSpace(payloadHash),
			"digest":             strings.TrimSpace(digest),
			"authority_boundary": "arvis_authoritative_reference_projection_only",
		},
	}
	observation, err := BuildGlobalRadarObservation(GlobalRadarObservationThreat, subject, evidence)
	if err != nil {
		return GlobalRadarObservation{}, time.Time{}, false, err
	}
	return observation, createdAt.UTC(), true, nil
}

// BridgeGlobalCampaignEvidenceOnce projects only already-verified evidence. Rows
// that cannot form canonical endpoint subjects are deliberately skipped and the
// cursor advances so unsupported evidence cannot wedge the worker.
func BridgeGlobalCampaignEvidenceOnce(ctx context.Context, db *sql.DB, sink GlobalRadarSnapshotSink, limit int) (int, int, error) {
	if db == nil {
		return 0, 0, ErrGlobalCampaignStoreUnavailable
	}
	if sink == nil {
		return 0, 0, errors.New("campaign evidence bridge sink is unavailable")
	}
	cursor, err := loadGlobalCampaignEvidenceCursor(ctx, db)
	if err != nil {
		return 0, 0, err
	}
	rows, err := readGlobalCampaignActorEvidence(ctx, db, cursor, limit)
	if err != nil {
		return 0, 0, err
	}
	processed, projected := 0, 0
	for _, row := range rows {
		next := globalCampaignEvidenceCursor{ObservedAt: row.ObservedAt.UTC(), EvidenceID: strings.TrimSpace(row.ID)}
		snapshot, eligible, err := buildGlobalCampaignActorEvidenceSnapshot(ctx, db, row)
		if err != nil {
			return processed, projected, err
		}
		if eligible {
			if err := sink.InsertGlobalRadarSnapshot(ctx, snapshot); err != nil {
				return processed, projected, fmt.Errorf("persist verified campaign evidence snapshot: %w", err)
			}
			projected++
		}
		if err := advanceGlobalCampaignEvidenceCursor(ctx, db, next); err != nil {
			return processed, projected, err
		}
		processed++
	}
	return processed, projected, nil
}

func StartGlobalCampaignEvidenceBridge(parent context.Context, db *sql.DB, sink GlobalRadarSnapshotSink, health *runtimehealth.Registry) func() {
	if db == nil || sink == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	if health != nil {
		health.RegisterPeriodic(GlobalCampaignEvidenceBridgeHealthID, "worker", "", true, time.Minute)
	}
	go func() {
		defer close(done)
		if health != nil {
			defer health.Stop(GlobalCampaignEvidenceBridgeHealthID)
		}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			cycle, cycleCancel := context.WithTimeout(ctx, 25*time.Second)
			processed, projected, err := BridgeGlobalCampaignEvidenceOnce(cycle, db, sink, 25)
			cycleCancel()
			if health != nil {
				if err != nil {
					health.Failure(GlobalCampaignEvidenceBridgeHealthID, err)
				} else {
					health.Success(GlobalCampaignEvidenceBridgeHealthID, processed+projected)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
