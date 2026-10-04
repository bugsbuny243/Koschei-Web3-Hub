package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"koschei/api/internal/runtimehealth"
)

const GlobalCampaignRuntimeHealthID = "worker.global-campaign-materializer"
const GlobalCampaignRuntimeVersion = "koschei.global-campaign-runtime.v1"
const globalCampaignRuntimeLeaseKey = "global-campaign-runtime.v1"
const globalCampaignRuntimeMaxObservations = 128

var ErrGlobalCampaignRuntimeInput = errors.New("invalid or oversized campaign runtime input")

type GlobalCampaignRuntimeConfig struct {
	DB     *sql.DB
	Owner  string
	Health *runtimehealth.Registry
}

// Only references and C6/C7 projections cross this boundary, never raw payloads.
type globalCampaignRuntimeJob struct {
	Version string                          `json:"version"`
	Input   GlobalCampaignMaterializerInput `json:"material"`
	Radar   GlobalCampaignRadarProjection   `json:"radar"`
}

type GlobalCampaignRuntimeSnapshot struct {
	Version     string                              `json:"version"`
	SourceRef   string                              `json:"source_ref"`
	Campaign    GlobalCampaign                      `json:"campaign"`
	Radar       GlobalCampaignRadarProjection       `json:"radar"`
	Fabric      FabricCampaignEvidenceContract      `json:"fabric"`
	Command     GlobalCampaignCommandCenterSnapshot `json:"command"`
	ProcessedAt time.Time                           `json:"processed_at"`
}

type GlobalCampaignRuntimeStatus struct {
	Enabled           bool       `json:"enabled"`
	Pending           int64      `json:"pending"`
	Failed            int64      `json:"failed"`
	Processed         int64      `json:"processed"`
	OldestPendingAt   *time.Time `json:"oldest_pending_at,omitempty"`
	LatestProcessedAt *time.Time `json:"latest_processed_at,omitempty"`
	Scope             string     `json:"scope"`
	AutomaticResponse bool       `json:"automatic_response"`
}

func VerifyGlobalCampaignRuntimeSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrGlobalCampaignStoreUnavailable
	}
	var ok bool
	err := db.QueryRowContext(ctx, `SELECT to_regclass('global_campaign_runtime_queue') IS NOT NULL
        AND to_regclass('global_campaign_current') IS NOT NULL
        AND to_regclass('global_campaign_anchor_index') IS NOT NULL
        AND to_regclass('global_campaign_worker_leases') IS NOT NULL`).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("campaign runtime migrations are required")
	}
	return nil
}

// Derivation requires a canonical VERIFIED graph link. Time proximity, a shared
// network, or independent block-head observations never create a campaign.
func buildGlobalCampaignRuntimeJobs(snapshot GlobalRadarSnapshot) ([]globalCampaignRuntimeJob, error) {
	if len(snapshot.Observations) > globalCampaignRuntimeMaxObservations || len(snapshot.Relations)+len(snapshot.BridgeLinks) > 256 {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	canonical, err := validateCanonicalGlobalCampaignRadarSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	groups := map[string]string{}
	var root func(string) string
	root = func(s string) string {
		p, exists := groups[s]
		if !exists {
			groups[s] = s
			return s
		}
		if p != s {
			groups[s] = root(p)
		}
		return groups[s]
	}
	for _, edge := range canonical.Relations {
		if edge.Status == IntelligenceEvidenceVerified && edge.Source.ID != edge.Target.ID {
			a, b := root(edge.Source.ID), root(edge.Target.ID)
			if a < b {
				groups[b] = a
			} else {
				groups[a] = b
			}
		}
	}
	observations := map[string][]GlobalRadarObservation{}
	for _, obs := range canonical.Observations {
		if _, linked := groups[obs.Subject.ID]; linked {
			key := root(obs.Subject.ID)
			observations[key] = append(observations[key], obs)
		}
	}
	keys := make([]string, 0, len(observations))
	for key := range observations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	jobs := make([]globalCampaignRuntimeJob, 0)
	for _, key := range keys {
		obs := observations[key]
		subjects, observationIDs := map[string]bool{}, map[string]bool{}
		for _, item := range obs {
			subjects[item.Subject.ID] = true
			observationIDs[item.ObservationID] = true
		}
		if len(subjects) < 2 {
			continue
		}
		edges, bridges := []GlobalRadarRelationEdge{}, []GlobalRadarBridgeLink{}
		for _, edge := range canonical.Relations {
			if subjects[edge.Source.ID] && subjects[edge.Target.ID] {
				edges = append(edges, edge)
			}
		}
		for _, link := range canonical.BridgeLinks {
			if observationIDs[link.SourceObservation.ObservationID] && observationIDs[link.DestinationObservation.ObservationID] {
				bridges = append(bridges, link)
			}
		}
		component, err := BuildGlobalRadarSnapshot(obs, edges, bridges, nil, canonical.GeneratedAt)
		if err != nil {
			return nil, err
		}
		in := GlobalCampaignMaterializerInput{RulesetVersion: GlobalCampaignRuntimeVersion,
			MissingEvidence: []string{"threat_family_inputs_not_connected", "operator_classification_required", "response_not_executed"}}
		for _, item := range component.Observations {
			in.Networks = append(in.Networks, item.Subject.Network)
			in.Subjects = append(in.Subjects, item.Subject.ID)
			in.ObservationRefs = append(in.ObservationRefs, item.ObservationID)
			if item.ObservedAt.After(in.ObservedAt) {
				in.ObservedAt = item.ObservedAt
			}
			if item.Evidence.Status == IntelligenceEvidenceVerified {
				in.VerifiedAnchorCount++
			} else {
				in.ObservedAnchorCount++
			}
		}
		for _, edge := range component.Relations {
			in.RelationRefs = append(in.RelationRefs, edge.EdgeID)
			if edge.Status == IntelligenceEvidenceVerified {
				in.VerifiedAnchorCount++
			} else {
				in.ObservedAnchorCount++
			}
		}
		for _, link := range component.BridgeLinks {
			in.BridgeLinkRefs = append(in.BridgeLinkRefs, link.LinkID)
			in.Bridges = append(in.Bridges, link.BridgeProtocol)
			in.VerifiedAnchorCount++
		}
		in.Networks = normalizeGlobalCampaignStrings(in.Networks)
		in.Subjects = normalizeGlobalCampaignStrings(in.Subjects)
		in.ObservationRefs = normalizeGlobalCampaignStrings(in.ObservationRefs)
		in.RelationRefs = normalizeGlobalCampaignStrings(in.RelationRefs)
		in.BridgeLinkRefs = normalizeGlobalCampaignStrings(in.BridgeLinkRefs)
		in.Bridges = normalizeGlobalCampaignStrings(in.Bridges)
		campaign := MaterializeGlobalCampaign(in)
		radar, err := BuildGlobalCampaignRadarProjection(campaign.CampaignRef, component)
		if err != nil {
			return nil, err
		}
		job := globalCampaignRuntimeJob{Version: GlobalCampaignRuntimeVersion, Input: in, Radar: radar}
		if _, err := validateGlobalCampaignRuntimeJob(job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
		if len(jobs) > 16 {
			return nil, ErrGlobalCampaignRuntimeInput
		}
	}
	return jobs, nil
}

func validateGlobalCampaignRuntimeJob(job globalCampaignRuntimeJob) ([]byte, error) {
	if job.Version != GlobalCampaignRuntimeVersion || job.Input.RulesetVersion != GlobalCampaignRuntimeVersion ||
		len(job.Radar.Events) < 2 || len(job.Radar.Events) > globalCampaignRuntimeMaxObservations ||
		len(job.Input.Subjects) < 2 || job.Radar.Version != GlobalCampaignRadarProjectionVersion ||
		job.Radar.FingerprintSHA256 != hashGlobalCampaignRadarProjection(job.Radar) ||
		job.Radar.VerdictAuthority || job.Radar.GradeAuthority || job.Radar.ContainmentAuthority ||
		job.Radar.SameOperatorClaim || job.Radar.RealWorldIdentityClaim || job.Radar.WrongdoingClaim {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	if err := validateGlobalCampaignManagedIdentityInput(job.Input); err != nil {
		return nil, err
	}
	refs, subjects, networks, verifiedRelations := []string{}, []string{}, []string{}, []string{}
	for _, event := range job.Radar.Events {
		refs = append(refs, event.Ref)
		subjects = append(subjects, event.SubjectID)
		networks = append(networks, event.Network)
		verifiedRelations = append(verifiedRelations, event.VerifiedRelationRefs...)
	}
	if !reflect.DeepEqual(normalizeGlobalCampaignStrings(refs), job.Input.ObservationRefs) ||
		!reflect.DeepEqual(normalizeGlobalCampaignStrings(subjects), job.Input.Subjects) ||
		!reflect.DeepEqual(normalizeGlobalCampaignStrings(networks), job.Input.Networks) || len(verifiedRelations) == 0 {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	temporal, err := BuildGlobalCampaignTemporalCorrelation(job.Radar.CampaignRef, job.Radar.Events)
	if err != nil || temporal.FingerprintSHA256 != job.Radar.Temporal.FingerprintSHA256 ||
		hashGlobalCampaignTemporalCorrelationReport(job.Radar.Temporal) != temporal.FingerprintSHA256 {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	campaign := MaterializeGlobalCampaign(job.Input)
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return nil, err
	}
	if campaign.CampaignRef != job.Radar.CampaignRef {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	if _, err := BuildFabricCampaignEvidenceContract(campaign, job.Radar.Temporal, job.Radar, GlobalCampaignThreatFamilyReport{}, nil); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(job)
	// Leave margin for PostgreSQL's whitespace in jsonb::text.
	if err != nil || len(payload) > 200000 {
		return nil, ErrGlobalCampaignRuntimeInput
	}
	return payload, nil
}

func globalCampaignRuntimeSourceRef(job globalCampaignRuntimeJob) string {
	// Generation time is transport metadata. Replays with identical canonical
	// evidence have the same source identity even if collected again later.
	body, _ := json.Marshal(struct {
		Input      GlobalCampaignMaterializerInput
		Events     []GlobalCampaignTemporalEvent
		Continuity []GlobalCampaignCrossNetworkContinuity
	}{job.Input, job.Radar.Events, job.Radar.CrossNetworkContinuity})
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func EnqueueGlobalCampaignRadarSnapshot(ctx context.Context, db *sql.DB, snapshot GlobalRadarSnapshot) (int, error) {
	if db == nil {
		return 0, ErrGlobalCampaignStoreUnavailable
	}
	jobs, err := buildGlobalCampaignRuntimeJobs(snapshot)
	if err != nil || len(jobs) == 0 {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Bound outstanding work across all producers, including separate API pods.
	var ignored any
	if err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('global-campaign-runtime-queue',0))`).Scan(&ignored); err != nil {
		return 0, err
	}
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM global_campaign_runtime_queue WHERE status='pending' LIMIT 10000) q`).Scan(&pending); err != nil {
		return 0, err
	}
	inserted := 0
	for _, job := range jobs {
		payload, err := validateGlobalCampaignRuntimeJob(job)
		if err != nil {
			return 0, err
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO global_campaign_runtime_queue(source_ref,reference_payload)
            VALUES ($1,$2) ON CONFLICT (source_ref) DO NOTHING`, globalCampaignRuntimeSourceRef(job), string(payload))
		if err != nil {
			return 0, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		inserted += int(n)
		if pending+inserted > 10000 {
			return 0, errors.New("campaign runtime queue capacity reached")
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// The lease row stays locked through materialization and acknowledgement.
// Checking the token outside this transaction would permit a stale writer.
func ProcessGlobalCampaignRuntimeJob(ctx context.Context, db *sql.DB, lease GlobalCampaignWorkerLease) (bool, error) {
	if db == nil {
		return false, ErrGlobalCampaignStoreUnavailable
	}
	if lease.LeaseKey != globalCampaignRuntimeLeaseKey {
		return false, ErrGlobalCampaignWorkerLeaseFenced
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	current, found, err := loadGlobalCampaignWorkerLeaseTx(ctx, tx, lease.LeaseKey, true)
	if err != nil {
		return false, err
	}
	var databaseNow time.Time
	if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow); err != nil {
		return false, err
	}
	if !found || current.LeaseOwner != lease.LeaseOwner || current.FencingToken != lease.FencingToken || !current.LeaseExpiresAt.After(databaseNow) {
		return false, ErrGlobalCampaignWorkerLeaseFenced
	}
	var source string
	var payload []byte
	err = tx.QueryRowContext(ctx, `SELECT source_ref,reference_payload FROM global_campaign_runtime_queue
        WHERE status='pending' ORDER BY created_at,source_ref LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&source, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var job globalCampaignRuntimeJob
	err = json.Unmarshal(payload, &job)
	if err == nil {
		_, err = validateGlobalCampaignRuntimeJob(job)
	}
	if err != nil || source != globalCampaignRuntimeSourceRef(job) {
		if _, updateErr := tx.ExecContext(ctx, `UPDATE global_campaign_runtime_queue SET status='failed',error_code='invalid_reference_payload',processed_at=clock_timestamp() WHERE source_ref=$1`, source); updateErr != nil {
			return false, updateErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return false, commitErr
		}
		return false, ErrGlobalCampaignRuntimeInput
	}
	if _, err := tx.ExecContext(ctx, `SAVEPOINT campaign_materialization`); err != nil {
		return false, err
	}
	result, err := materializeAndPersistGlobalCampaignTx(ctx, tx, job.Input)
	if err != nil {
		// Deterministic conflicts require operator resolution. Quarantine them
		// after rolling back all campaign writes so other evidence can proceed.
		if errors.Is(err, ErrGlobalCampaignAmbiguousCandidates) || errors.Is(err, ErrGlobalCampaignRulesetConflict) {
			if _, rollbackErr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT campaign_materialization`); rollbackErr != nil {
				return false, rollbackErr
			}
			if _, updateErr := tx.ExecContext(ctx, `UPDATE global_campaign_runtime_queue SET status='failed',error_code='campaign_evidence_conflict',processed_at=clock_timestamp() WHERE source_ref=$1`, source); updateErr != nil {
				return false, updateErr
			}
			if commitErr := tx.Commit(); commitErr != nil {
				return false, commitErr
			}
		}
		return false, err
	}
	radar := job.Radar
	radar.CampaignRef = result.Campaign.CampaignRef
	radar.Temporal, err = BuildGlobalCampaignTemporalCorrelation(radar.CampaignRef, radar.Events)
	if err != nil {
		return false, err
	}
	radar.FingerprintSHA256 = hashGlobalCampaignRadarProjection(radar)
	fabric, err := BuildFabricCampaignEvidenceContract(result.Campaign, radar.Temporal, radar, GlobalCampaignThreatFamilyReport{}, nil)
	if err != nil {
		return false, err
	}
	command, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign: result.Campaign, Fabric: fabric, ObservedAt: result.Campaign.LastObservedAt})
	if err != nil {
		return false, err
	}
	snapshot := GlobalCampaignRuntimeSnapshot{Version: GlobalCampaignRuntimeVersion, SourceRef: source,
		Campaign: result.Campaign, Radar: radar, Fabric: fabric, Command: command, ProcessedAt: databaseNow.UTC()}
	output, err := json.Marshal(snapshot)
	if err != nil {
		return false, err
	}
	ack, err := tx.ExecContext(ctx, `UPDATE global_campaign_runtime_queue
        SET status='processed',campaign_ref=$2,campaign_revision=$3,command_payload=$4,processed_at=clock_timestamp()
        WHERE source_ref=$1 AND EXISTS (SELECT 1 FROM global_campaign_worker_leases
            WHERE lease_key=$5 AND lease_owner=$6 AND fencing_token=$7 AND lease_expires_at>clock_timestamp())`,
		source, result.Campaign.CampaignRef, result.Campaign.Revision, string(output), lease.LeaseKey, lease.LeaseOwner, lease.FencingToken)
	if err != nil {
		return false, err
	}
	n, err := ack.RowsAffected()
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, ErrGlobalCampaignWorkerLeaseFenced
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func StartGlobalCampaignRuntime(parent context.Context, cfg GlobalCampaignRuntimeConfig) func() {
	if cfg.DB == nil || cfg.Owner == "" {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	cfg.Health.RegisterPeriodic(GlobalCampaignRuntimeHealthID, "worker", "", true, time.Minute)
	go func() {
		defer close(done)
		defer cfg.Health.Stop(GlobalCampaignRuntimeHealthID)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var lease GlobalCampaignWorkerLease
		defer func() {
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer releaseCancel()
			if lease.FencingToken > 0 {
				_ = ReleaseGlobalCampaignWorkerLease(releaseCtx, cfg.DB, lease, time.Now().UTC())
			}
		}()
		for {
			if ctx.Err() != nil {
				return
			}
			cycle, cycleCancel := context.WithTimeout(ctx, 25*time.Second)
			var err error
			var now time.Time
			err = cfg.DB.QueryRowContext(cycle, `SELECT clock_timestamp()`).Scan(&now)
			if err == nil {
				if lease.FencingToken > 0 && lease.LeaseExpiresAt.After(now) {
					lease, err = RenewGlobalCampaignWorkerLease(cycle, cfg.DB, lease, now, time.Minute)
				} else {
					lease, err = AcquireGlobalCampaignWorkerLease(cycle, cfg.DB, globalCampaignRuntimeLeaseKey, cfg.Owner, now, time.Minute)
				}
			}
			processed := 0
			for err == nil && processed < 5 {
				var worked bool
				worked, err = ProcessGlobalCampaignRuntimeJob(cycle, cfg.DB, lease)
				if !worked {
					break
				}
				processed++
			}
			if err != nil {
				cfg.Health.Failure(GlobalCampaignRuntimeHealthID, err)
			} else {
				cfg.Health.Success(GlobalCampaignRuntimeHealthID, processed)
			}
			cycleCancel()
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

func ReadGlobalCampaignRuntime(ctx context.Context, db *sql.DB, enabled bool) (GlobalCampaignRuntimeStatus, []GlobalCampaignRuntimeSnapshot, error) {
	status := GlobalCampaignRuntimeStatus{Enabled: enabled, Scope: "persisted_campaigns_latest_source_window; missing_evidence=unknown"}
	snapshots := []GlobalCampaignRuntimeSnapshot{}
	if db == nil {
		return status, snapshots, ErrGlobalCampaignStoreUnavailable
	}
	err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE status='pending'),count(*) FILTER (WHERE status='failed'),
        count(*) FILTER (WHERE status='processed'), min(created_at) FILTER (WHERE status='pending'),max(processed_at) FILTER (WHERE status='processed')
        FROM global_campaign_runtime_queue`).Scan(&status.Pending, &status.Failed, &status.Processed, &status.OldestPendingAt, &status.LatestProcessedAt)
	if err != nil {
		return status, snapshots, err
	}
	// Pin to the current immutable revision. A newer externally persisted
	// revision with no runtime projection must never display an older view.
	rows, err := db.QueryContext(ctx, `SELECT q.source_ref,q.command_payload,c.campaign_ref,c.revision,c.evidence_hash_sha256 FROM global_campaign_current c
		JOIN LATERAL (SELECT source_ref,command_payload FROM global_campaign_runtime_queue
            WHERE campaign_ref=c.campaign_ref AND campaign_revision=c.revision AND status='processed'
            ORDER BY processed_at DESC LIMIT 1) q ON true
        ORDER BY c.last_observed_at DESC,c.campaign_ref LIMIT 25`)
	if err != nil {
		return status, snapshots, err
	}
	defer rows.Close()
	for rows.Next() {
		var payload []byte
		var sourceRef, campaignRef, evidenceHash string
		var revision uint64
		if err := rows.Scan(&sourceRef, &payload, &campaignRef, &revision, &evidenceHash); err != nil {
			return status, nil, err
		}
		var snapshot GlobalCampaignRuntimeSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return status, nil, ErrGlobalCampaignStoredCorrupt
		}
		if snapshot.SourceRef != sourceRef || snapshot.Campaign.CampaignRef != campaignRef ||
			snapshot.Campaign.Revision != revision || snapshot.Campaign.EvidenceHashSHA256 != evidenceHash {
			return status, nil, ErrGlobalCampaignStoredCorrupt
		}
		if err := validateGlobalCampaignRuntimeSnapshot(snapshot); err != nil {
			return status, nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return status, snapshots, rows.Err()
}

func validateGlobalCampaignRuntimeSnapshot(snapshot GlobalCampaignRuntimeSnapshot) error {
	if snapshot.Version != GlobalCampaignRuntimeVersion || snapshot.SourceRef == "" || snapshot.ProcessedAt.IsZero() {
		return ErrGlobalCampaignStoredCorrupt
	}
	if err := ValidateGlobalCampaign(snapshot.Campaign); err != nil {
		return err
	}
	if err := ValidateFabricCampaignEvidenceContract(snapshot.Fabric); err != nil {
		return err
	}
	if err := ValidateGlobalCampaignCommandCenterSnapshot(snapshot.Command); err != nil {
		return err
	}
	if snapshot.Radar.FingerprintSHA256 != hashGlobalCampaignRadarProjection(snapshot.Radar) {
		return ErrGlobalCampaignStoredCorrupt
	}
	fabric, err := BuildFabricCampaignEvidenceContract(snapshot.Campaign, snapshot.Radar.Temporal, snapshot.Radar, GlobalCampaignThreatFamilyReport{}, nil)
	if err != nil {
		return err
	}
	command, err := BuildGlobalCampaignCommandCenterSnapshot(GlobalCampaignCommandCenterInput{
		Campaign: snapshot.Campaign, Fabric: fabric, ObservedAt: snapshot.Campaign.LastObservedAt})
	if err != nil {
		return err
	}
	if fabric.ContractHashSHA256 != snapshot.Fabric.ContractHashSHA256 || command.SnapshotHashSHA256 != snapshot.Command.SnapshotHashSHA256 {
		return fmt.Errorf("%w: runtime components mismatch", ErrGlobalCampaignStoredCorrupt)
	}
	return nil
}
