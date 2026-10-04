package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	globalCampaignRadarLeaseKey        = "security-radar-to-global-campaign-v1"
	defaultGlobalCampaignRadarInterval = time.Minute
	defaultGlobalCampaignRadarBatch    = 50
	globalCampaignRadarLeaseTTL        = 5 * time.Minute
)

var ErrGlobalCampaignRadarSubjectUnsupported = errors.New("security radar verdict target cannot be represented as a canonical Global Radar subject")

type globalCampaignRadarRuntimeVerdict struct {
	SecurityRadarVerdictRecord
	EvidenceBacked bool
}

// StartGlobalCampaignRadarRuntime connects the signed/evidence-backed Security
// Radar verdict stream to canonical Global Radar observations and then to the
// Global Campaign correlation store. The worker never creates a new verdict,
// grade, identity claim, common-control claim, or containment authority.
func StartGlobalCampaignRadarRuntime(ctx context.Context, db *sql.DB) func() {
	if db == nil || !globalCampaignRadarRuntimeEnabled() {
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	go runGlobalCampaignRadarRuntime(workerCtx, db)
	log.Printf("global campaign radar runtime started: interval=%s batch=%d authority=correlation_only", globalCampaignRadarInterval(), globalCampaignRadarBatchSize())
	return cancel
}

func runGlobalCampaignRadarRuntime(ctx context.Context, db *sql.DB) {
	owner := globalCampaignRadarLeaseOwner()
	interval := globalCampaignRadarInterval()
	batch := globalCampaignRadarBatchSize()
	var lease GlobalCampaignWorkerLease

	defer func() {
		if strings.TrimSpace(lease.LeaseKey) == "" {
			return
		}
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := ReleaseGlobalCampaignWorkerLease(releaseCtx, db, lease, time.Now().UTC()); err != nil &&
			!errors.Is(err, ErrGlobalCampaignWorkerLeaseExpired) && !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
			log.Printf("global campaign radar lease release failed: %v", err)
		}
	}()

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		now := time.Now().UTC()
		var err error
		if strings.TrimSpace(lease.LeaseKey) == "" {
			lease, err = AcquireGlobalCampaignWorkerLease(ctx, db, globalCampaignRadarLeaseKey, owner, now, globalCampaignRadarLeaseTTL)
		} else {
			lease, err = RenewGlobalCampaignWorkerLease(ctx, db, lease, now, globalCampaignRadarLeaseTTL)
		}
		if err != nil {
			if !errors.Is(err, ErrGlobalCampaignWorkerLeaseBusy) && !errors.Is(err, ErrGlobalCampaignWorkerLeaseFenced) {
				log.Printf("global campaign radar lease unavailable: %v", err)
			}
			lease = GlobalCampaignWorkerLease{}
			timer.Reset(interval)
			continue
		}

		runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		projected, skipped, runErr := runGlobalCampaignRadarCycle(runCtx, db, lease, batch)
		cancel()
		if runErr != nil && ctx.Err() == nil {
			log.Printf("global campaign radar cycle failed: %v", runErr)
		} else if projected > 0 || skipped > 0 {
			log.Printf("global campaign radar cycle: projected=%d skipped=%d", projected, skipped)
		}
		timer.Reset(interval)
	}
}

func runGlobalCampaignRadarCycle(ctx context.Context, db *sql.DB, lease GlobalCampaignWorkerLease, limit int) (int, int, error) {
	if err := AssertGlobalCampaignWorkerLease(ctx, db, lease, time.Now().UTC()); err != nil {
		return 0, 0, err
	}
	items, err := loadPendingGlobalCampaignRadarVerdicts(ctx, db, limit)
	if err != nil {
		return 0, 0, err
	}
	projected := 0
	skipped := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return projected, skipped, ctx.Err()
		}
		if err := AssertGlobalCampaignWorkerLease(ctx, db, lease, time.Now().UTC()); err != nil {
			return projected, skipped, err
		}
		observation, snapshot, material, err := projectSecurityRadarVerdictToGlobalCampaign(item)
		if err != nil {
			if errors.Is(err, ErrGlobalCampaignRadarSubjectUnsupported) {
				if markErr := markGlobalCampaignRadarSkipped(ctx, db, item.ID, err.Error()); markErr != nil {
					return projected, skipped, markErr
				}
				skipped++
				continue
			}
			return projected, skipped, err
		}

		preview := MaterializeGlobalCampaign(material)
		if strings.TrimSpace(preview.CampaignRef) == "" {
			return projected, skipped, errors.New("global campaign radar projection produced empty campaign identity")
		}
		projection, err := BuildGlobalCampaignRadarProjection(preview.CampaignRef, snapshot)
		if err != nil {
			return projected, skipped, fmt.Errorf("build campaign radar projection verdict=%s: %w", item.ID, err)
		}
		result, err := MaterializeAndPersistGlobalCampaign(ctx, db, material)
		if err != nil {
			return projected, skipped, fmt.Errorf("persist campaign radar projection verdict=%s: %w", item.ID, err)
		}
		if err := markGlobalCampaignRadarProjected(ctx, db, item.ID, result.Campaign.CampaignRef, observation.ObservationID, projection.FingerprintSHA256); err != nil {
			return projected, skipped, err
		}
		projected++
	}
	return projected, skipped, nil
}

func loadPendingGlobalCampaignRadarVerdicts(ctx context.Context, db *sql.DB, limit int) ([]globalCampaignRadarRuntimeVerdict, error) {
	if limit <= 0 || limit > 500 {
		limit = defaultGlobalCampaignRadarBatch
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
			v.id::text,
			COALESCE(v.event_id::text,''),
			v.module_id,
			v.target,
			v.target_type,
			v.network,
			v.grade,
			v.risk_index,
			v.risk_level,
			v.verdict,
			v.recommendation,
			v.evidence,
			v.signals,
			v.rule_version,
			COALESCE(v.evidence_verified,false),
			COALESCE(v.digest,''),
			v.signed,
			COALESCE(v.signature,''),
			COALESCE(v.signature_algorithm,''),
			COALESCE(v.key_id,''),
			COALESCE(v.payload_hash,''),
			COALESCE(v.source,''),
			v.created_at,
			(
				COALESCE(v.evidence_verified,false)
				OR COALESCE(v.signals->>'verified_evidence','false')='true'
				OR COALESCE(v.signals->>'real_onchain_evidence','false')='true'
				OR COALESCE(v.signals->>'real_offchain_evidence','false')='true'
			) AS evidence_backed
		FROM security_radar_verdicts v
		WHERE v.module_id='final_verdict_engine'
		  AND v.signed=true
		  AND v.signature IS NOT NULL
		  AND btrim(v.signature)<>''
		  AND btrim(v.target)<>''
		  AND NOT EXISTS (
			SELECT 1 FROM global_campaign_radar_ingest i WHERE i.verdict_id=v.id
		  )
		  AND (
			COALESCE(v.evidence_verified,false)
			OR COALESCE(v.signals->>'verified_evidence','false')='true'
			OR COALESCE(v.signals->>'real_onchain_evidence','false')='true'
			OR COALESCE(v.signals->>'real_offchain_evidence','false')='true'
		  )
		ORDER BY v.created_at ASC, v.id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("load pending global campaign radar verdicts: %w", err)
	}
	defer rows.Close()

	items := make([]globalCampaignRadarRuntimeVerdict, 0, limit)
	for rows.Next() {
		var item globalCampaignRadarRuntimeVerdict
		var evidenceRaw, signalsRaw []byte
		if err := rows.Scan(
			&item.ID, &item.EventID, &item.ModuleID, &item.Target, &item.TargetType, &item.Network,
			&item.Grade, &item.RiskIndex, &item.RiskLevel, &item.Verdict, &item.Recommendation,
			&evidenceRaw, &signalsRaw, &item.RuleVersion, &item.EvidenceVerified, &item.Digest,
			&item.Signed, &item.Signature, &item.SignatureAlgorithm, &item.KeyID, &item.PayloadHash,
			&item.Source, &item.CreatedAt, &item.EvidenceBacked,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(evidenceRaw, &item.Evidence)
		_ = json.Unmarshal(signalsRaw, &item.Signals)
		items = append(items, item)
	}
	return items, rows.Err()
}

func projectSecurityRadarVerdictToGlobalCampaign(item globalCampaignRadarRuntimeVerdict) (GlobalRadarObservation, GlobalRadarSnapshot, GlobalCampaignMaterializerInput, error) {
	subject := ClassifyIntelligenceSubject(item.Target, normalizeRadarNetwork(item.Network))
	if subject.ChainFamily == IntelligenceChainFamilyUnknown || subject.Kind == IntelligenceSubjectUnknown {
		return GlobalRadarObservation{}, GlobalRadarSnapshot{}, GlobalCampaignMaterializerInput{}, fmt.Errorf("%w: verdict=%s network=%s target=%s", ErrGlobalCampaignRadarSubjectUnsupported, item.ID, item.Network, item.Target)
	}
	if item.CreatedAt.IsZero() || strings.TrimSpace(item.ID) == "" {
		return GlobalRadarObservation{}, GlobalRadarSnapshot{}, GlobalCampaignMaterializerInput{}, errors.New("security radar verdict requires durable identity and timestamp")
	}

	evidenceStatus := IntelligenceEvidenceObserved
	if item.EvidenceVerified || securityRadarBoolSignal(item.Signals, "verified_evidence") {
		evidenceStatus = IntelligenceEvidenceVerified
	}
	identityMaterial := firstSecurityRadarString(item.PayloadHash, firstSecurityRadarString(item.Digest, item.ID))
	evidence := IntelligenceEvidence{
		ID:          intelligenceStableID("security-radar-verdict-evidence:" + item.ID + ":" + identityMaterial),
		SubjectID:   subject.ID,
		ChainFamily: subject.ChainFamily,
		Chain:       subject.Chain,
		Network:     subject.Network,
		Source:      "security_radar",
		Status:      evidenceStatus,
		ObservedAt:  item.CreatedAt.UTC(),
		Address:     subject.Raw,
		Provenance:  firstSecurityRadarString(item.Source, "security_radar"),
		Confidence:  0,
		Attributes: map[string]any{
			"security_radar_verdict_id": item.ID,
			"module_id":                 item.ModuleID,
			"grade":                     item.Grade,
			"risk_index":                item.RiskIndex,
			"risk_level":                item.RiskLevel,
			"rule_version":              item.RuleVersion,
			"source":                    item.Source,
		},
	}
	if evidenceStatus == IntelligenceEvidenceVerified {
		evidence.Confidence = 1
	}
	observation, err := BuildGlobalRadarObservation(GlobalRadarObservationThreat, subject, evidence)
	if err != nil {
		return GlobalRadarObservation{}, GlobalRadarSnapshot{}, GlobalCampaignMaterializerInput{}, err
	}
	snapshot, err := BuildGlobalRadarSnapshot([]GlobalRadarObservation{observation}, nil, nil, nil, item.CreatedAt.UTC())
	if err != nil {
		return GlobalRadarObservation{}, GlobalRadarSnapshot{}, GlobalCampaignMaterializerInput{}, err
	}

	material := GlobalCampaignMaterializerInput{
		ObservedAt:      item.CreatedAt.UTC(),
		Networks:        []string{subject.Network},
		Subjects:        []string{subject.ID},
		ObservationRefs: []string{observation.ObservationID},
		VerdictRefs:     []string{"security-radar-verdict:" + item.ID},
		RulesetVersion:  strings.TrimSpace(item.RuleVersion),
	}
	if evidenceStatus == IntelligenceEvidenceVerified {
		material.VerifiedAnchorCount = 1
	} else {
		material.ObservedAnchorCount = 1
	}
	switch strings.ToLower(strings.TrimSpace(item.TargetType)) {
	case "wallet", "address", "creator", "deployer":
		material.Actors = []string{subject.ID}
	case "token", "mint", "asset":
		material.Assets = []string{subject.ID}
	case "contract", "program":
		material.Contracts = []string{subject.ID}
	case "pool":
		material.Pools = []string{subject.ID}
	}
	return observation, snapshot, material, nil
}

func markGlobalCampaignRadarProjected(ctx context.Context, db *sql.DB, verdictID, campaignRef, observationRef, fingerprint string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO global_campaign_radar_ingest
		(verdict_id,status,campaign_ref,observation_ref,projection_fingerprint,skip_reason,processed_at)
		VALUES ($1::uuid,'projected',$2,$3,$4,NULL,now())
		ON CONFLICT (verdict_id) DO NOTHING`, verdictID, campaignRef, observationRef, fingerprint)
	if err != nil {
		return fmt.Errorf("record global campaign radar projection: %w", err)
	}
	return nil
}

func markGlobalCampaignRadarSkipped(ctx context.Context, db *sql.DB, verdictID, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > 1000 {
		reason = reason[:1000]
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO global_campaign_radar_ingest
		(verdict_id,status,campaign_ref,observation_ref,projection_fingerprint,skip_reason,processed_at)
		VALUES ($1::uuid,'skipped',NULL,NULL,NULL,$2,now())
		ON CONFLICT (verdict_id) DO NOTHING`, verdictID, reason)
	if err != nil {
		return fmt.Errorf("record skipped global campaign radar verdict: %w", err)
	}
	return nil
}

func securityRadarBoolSignal(signals map[string]any, key string) bool {
	if len(signals) == 0 {
		return false
	}
	value, ok := signals[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return err == nil && parsed
	default:
		return false
	}
}

func globalCampaignRadarRuntimeEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED"))
	if raw != "" {
		value, err := strconv.ParseBool(raw)
		return err == nil && value
	}
	return AutomaticBackgroundScanningEnabled()
}

func globalCampaignRadarInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_CAMPAIGN_INTERVAL"))
	if raw == "" {
		return defaultGlobalCampaignRadarInterval
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 15*time.Second || value > 15*time.Minute {
		return defaultGlobalCampaignRadarInterval
	}
	return value
}

func globalCampaignRadarBatchSize() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_CAMPAIGN_BATCH_SIZE")))
	if err != nil || value <= 0 || value > 500 {
		return defaultGlobalCampaignRadarBatch
	}
	return value
}

func globalCampaignRadarLeaseOwner() string {
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}
