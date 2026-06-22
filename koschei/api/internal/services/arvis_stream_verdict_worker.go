package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

var errArvisStreamInsufficientEvidence = errors.New("arvis stream insufficient evidence")

type arvisStreamTarget struct {
	ID        string
	Target    string
	Signature string
	Network   string
	Slot      int64
	ProgramID string
	ModuleID  string
}

func StartArvisStreamVerdictWorker(ctx context.Context, db *sql.DB) func() {
	if db == nil || envBool("ARVIS_STREAM_VERDICT_DISABLED") {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	go (&arvisStreamVerdictWorker{
		db:          db,
		store:       NewSecurityRadarStore(db),
		interval:    arvisStreamVerdictInterval(),
		batchSize:   arvisStreamVerdictBatchSize(),
		concurrency: arvisStreamVerdictConcurrency(),
	}).start(ctx)
	return cancel
}

type arvisStreamVerdictWorker struct {
	db          *sql.DB
	store       *SecurityRadarStore
	interval    time.Duration
	batchSize   int
	concurrency int
}

func (w *arvisStreamVerdictWorker) start(ctx context.Context) {
	if w == nil || w.db == nil || w.store == nil {
		return
	}
	if w.batchSize <= 0 {
		w.batchSize = arvisStreamVerdictBatchSize()
	}
	if w.concurrency <= 0 {
		w.concurrency = arvisStreamVerdictConcurrency()
	}
	log.Printf("arvis stream verdict worker started interval=%s batch=%d concurrency=%d", w.interval, w.batchSize, w.concurrency)
	w.processBatch(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("arvis stream verdict worker stopped")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *arvisStreamVerdictWorker) processBatch(ctx context.Context) {
	targets, err := w.pendingTargets(ctx, w.batchSize)
	if err != nil {
		if !isUndefinedTableError(err) {
			log.Printf("arvis stream verdict queue read failed: %v", err)
		}
		return
	}
	if len(targets) == 0 {
		return
	}
	concurrency := w.concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}

	jobs := make(chan arvisStreamTarget)
	var workers sync.WaitGroup
	workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer workers.Done()
			for target := range jobs {
				w.processOne(ctx, target)
			}
		}()
	}

sendLoop:
	for _, target := range targets {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- target:
		}
	}
	close(jobs)
	workers.Wait()
}

func (w *arvisStreamVerdictWorker) processOne(ctx context.Context, target arvisStreamTarget) {
	if !w.claimTarget(ctx, target) {
		return
	}
	err := w.processTarget(ctx, target)
	if errors.Is(err, errArvisStreamInsufficientEvidence) {
		w.markTarget(ctx, target.ID, "insufficient_evidence", SecurityRadarInsufficientEvidenceMessage)
		return
	}
	if err != nil {
		w.markTarget(ctx, target.ID, "failed", formatArvisWorkerError(err))
		log.Printf("arvis stream verdict failed event=%s target=%s module=%s err=%v", target.ID, target.Target, target.ModuleID, err)
		return
	}
	w.markTarget(ctx, target.ID, "completed", "")
}

func (w *arvisStreamVerdictWorker) pendingTargets(ctx context.Context, limit int) ([]arvisStreamTarget, error) {
	if limit <= 0 || limit > 100 {
		limit = arvisStreamVerdictBatchSize()
	}
	rows, err := w.db.QueryContext(ctx, `
		WITH eligible AS (
			SELECT s.id::text,
			       COALESCE(s.target,'') AS target,
			       COALESCE(s.signature,'') AS signature,
			       COALESCE(s.network,'solana-mainnet') AS network,
			       COALESCE(s.slot,0) AS slot,
			       COALESCE(s.program_id,'') AS program_id,
			       COALESCE(s.module_id,'') AS module_id,
			       s.created_at,
			       p.updated_at,
			       CASE
			         WHEN p.status='failed' THEN 'failed'
			         WHEN p.status='exhausted' THEN 'exhausted'
			         WHEN p.stream_event_id IS NULL AND s.created_at >= now() - interval '2 minutes' THEN 'fresh'
			         ELSE 'backlog'
			       END AS bucket,
			       COALESCE(p.updated_at,s.created_at) AS queue_time
			FROM security_radar_stream_events s
			LEFT JOIN arvis_stream_processing p ON p.stream_event_id=s.id
			WHERE COALESCE(s.target,'') <> ''
			  AND COALESCE(s.target_type,'')='token'
			  AND COALESCE(s.module_id,'') IN ($2,$3)
			  AND (
				p.stream_event_id IS NULL
				OR (p.status='failed' AND p.attempts < 3 AND p.updated_at < now() - interval '30 seconds')
				OR (p.status='exhausted' AND p.attempts < 5 AND p.updated_at < now() - interval '30 minutes')
			  )
		), ranked AS (
			SELECT eligible.*,
			       row_number() OVER (
			         PARTITION BY bucket
			         ORDER BY
			           CASE WHEN bucket='fresh' THEN created_at END DESC,
			           CASE WHEN bucket<>'fresh' THEN queue_time END ASC,
			           created_at ASC
			       ) AS bucket_rank
			FROM eligible
		), prioritized AS (
			SELECT ranked.*,
			       CASE
			         WHEN bucket='failed' AND bucket_rank <= GREATEST(1,$1/5) THEN 0
			         WHEN bucket='fresh' AND bucket_rank <= GREATEST(1,($1*3)/10) THEN 1
			         WHEN bucket='backlog' AND bucket_rank <= GREATEST(1,($1*4)/10) THEN 2
			         WHEN bucket='exhausted' AND bucket_rank <= GREATEST(1,$1/10) THEN 3
			         ELSE 10
			       END AS queue_priority
			FROM ranked
		)
		SELECT id,target,signature,network,slot,program_id,module_id
		FROM prioritized
		ORDER BY
		  queue_priority,
		  CASE WHEN bucket='fresh' THEN created_at END DESC,
		  CASE WHEN bucket<>'fresh' THEN queue_time END ASC,
		  created_at ASC
		LIMIT $1
	`, limit, ModulePumpSybilRadar, ModuleRaydiumPoolGuardian)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []arvisStreamTarget{}
	for rows.Next() {
		var item arvisStreamTarget
		if err := rows.Scan(&item.ID, &item.Target, &item.Signature, &item.Network, &item.Slot, &item.ProgramID, &item.ModuleID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (w *arvisStreamVerdictWorker) claimTarget(ctx context.Context, target arvisStreamTarget) bool {
	var attempts int
	err := w.db.QueryRowContext(ctx, `
		INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts,last_error,created_at,updated_at)
		VALUES($1::uuid,$2,$3,'processing',1,'',now(),now())
		ON CONFLICT(stream_event_id) DO UPDATE SET
			status='processing',
			attempts=arvis_stream_processing.attempts+1,
			last_error='',
			updated_at=now()
		WHERE (arvis_stream_processing.status='failed' AND arvis_stream_processing.attempts < 3)
		   OR (arvis_stream_processing.status='exhausted' AND arvis_stream_processing.attempts < 5)
		RETURNING attempts
	`, target.ID, target.Target, target.Signature).Scan(&attempts)
	return err == nil && attempts > 0
}

func (w *arvisStreamVerdictWorker) streamArmAlreadySaved(ctx context.Context, streamEventID, moduleID string) (bool, error) {
	if w == nil || w.db == nil || strings.TrimSpace(streamEventID) == "" || strings.TrimSpace(moduleID) == "" {
		return false, nil
	}
	var exists bool
	err := w.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM security_radar_verdicts
			WHERE source='arvis_stream'
			  AND module_id=$1
			  AND COALESCE(signals->>'source_stream_event_id','')=$2
		)
	`, moduleID, streamEventID).Scan(&exists)
	return exists, err
}

func arvisStreamAnalysisMode(moduleID string) string {
	moduleID = strings.TrimSpace(moduleID)
	switch moduleID {
	case ModulePumpSybilRadar, ModuleRaydiumPoolGuardian:
		return "live_stream:" + moduleID
	default:
		return "live_stream"
	}
}

func prioritizeFinalArvisArm(arms []SecurityRadarVerdict) []SecurityRadarVerdict {
	ordered := make([]SecurityRadarVerdict, 0, len(arms))
	for _, arm := range arms {
		if arm.ModuleID == ModuleFinalVerdictEngine {
			ordered = append(ordered, arm)
		}
	}
	for _, arm := range arms {
		if arm.ModuleID != ModuleFinalVerdictEngine {
			ordered = append(ordered, arm)
		}
	}
	return ordered
}

func (w *arvisStreamVerdictWorker) processTarget(ctx context.Context, target arvisStreamTarget) error {
	mode := arvisStreamAnalysisMode(target.ModuleID)
	analysis := AnalyzeArvisRadars(SecurityRadarRequest{Target: target.Target, Network: target.Network, Mode: mode})
	for i := range analysis.Arms {
		if analysis.Arms[i].Signals == nil {
			analysis.Arms[i].Signals = map[string]any{}
		}
		analysis.Arms[i].Signals["source_module"] = target.ModuleID
		analysis.Arms[i].Signals["source_stream_event_id"] = target.ID
		analysis.Arms[i].Signals["source_stream_signature"] = target.Signature
		analysis.Arms[i].Signals["source_program_id"] = target.ProgramID
		if strings.TrimSpace(target.Signature) != "" {
			analysis.Arms[i].Signals["latest_signature"] = target.Signature
		}
	}
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = analysis.Arms
	analysis.Bundle.Metadata["source_module"] = target.ModuleID
	analysis.Bundle.Metadata["source_stream_event_id"] = target.ID
	analysis.Bundle.Metadata["source_stream_signature"] = target.Signature
	analysis.Bundle.Metadata["source_program_id"] = target.ProgramID

	bundle := EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	final := ArvisFinalFromBundle(bundle)
	if !SecurityRadarHasLiveEvidence(bundle) || !final.Signed {
		return errArvisStreamInsufficientEvidence
	}
	arms := ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		return errors.New("arvis arms missing from bundle")
	}

	finalSaved := false
	auxiliaryFailures := []string{}
	for _, arm := range prioritizeFinalArvisArm(arms) {
		if !SecurityRadarVerdictHasVerifiedEvidence(arm) {
			continue
		}
		isFinal := arm.ModuleID == ModuleFinalVerdictEngine
		alreadySaved, err := w.streamArmAlreadySaved(ctx, target.ID, arm.ModuleID)
		if err != nil {
			wrapped := fmt.Errorf("check existing arm verdict %s: %w", arm.ModuleID, err)
			if isFinal {
				return wrapped
			}
			auxiliaryFailures = append(auxiliaryFailures, wrapped.Error())
			continue
		}
		if alreadySaved {
			if isFinal {
				finalSaved = true
			}
			continue
		}
		signals := cloneArvisSignals(arm.Signals)
		signals["source_module"] = target.ModuleID
		signals["source_stream_event_id"] = target.ID
		signals["source_stream_signature"] = target.Signature
		signals["source_program_id"] = target.ProgramID
		signals["stream_mode"] = mode
		provider := strings.TrimSpace(anyString(signals["provider"]))
		if provider == "" || provider == "none" || provider == "unconfigured" {
			provider = "solana_rpc"
		}
		eventID, err := w.store.InsertEvent(ctx, SecurityRadarEventRecord{
			ModuleID: arm.ModuleID, Target: arm.Target, TargetType: "token", Network: arm.Network,
			Signature: arm.Signature, SourceAddress: target.ProgramID, EventType: "arvis_stream_verdict",
			Slot: target.Slot, Signals: signals,
			RawSummary: map[string]any{
				"source_module":           target.ModuleID,
				"source_stream_event_id":  target.ID,
				"source_stream_signature": target.Signature,
				"source_program_id":       target.ProgramID,
			},
			Source: "arvis_stream",
		})
		if err != nil {
			wrapped := fmt.Errorf("insert arm event %s: %w", arm.ModuleID, err)
			if isFinal {
				return wrapped
			}
			auxiliaryFailures = append(auxiliaryFailures, wrapped.Error())
			continue
		}
		if _, err := w.store.InsertVerdict(ctx, SecurityRadarVerdictRecord{
			EventID: eventID, ModuleID: arm.ModuleID, Target: arm.Target, TargetType: "token", Network: arm.Network,
			Grade: arm.Grade, RiskIndex: arm.RiskIndex, RiskLevel: arm.RiskLevel, Verdict: arm.Verdict,
			Recommendation: arm.Recommendation, Evidence: arm.Evidence, Signals: signals,
			RuleVersion: arm.RuleVersion, Signed: arm.Signed, Signature: arm.Signature,
			Source: "arvis_stream", EventType: "arvis_stream_verdict", Provider: provider,
		}); err != nil {
			if isArvisUniqueViolation(err) {
				if isFinal {
					finalSaved = true
				}
				continue
			}
			wrapped := fmt.Errorf("insert arm verdict %s: %w", arm.ModuleID, err)
			if isFinal {
				return wrapped
			}
			auxiliaryFailures = append(auxiliaryFailures, wrapped.Error())
			continue
		}
		if isFinal {
			finalSaved = true
		}
	}
	if !finalSaved {
		return errors.New("final Arvis verdict was not persisted")
	}
	if len(auxiliaryFailures) > 0 {
		log.Printf("arvis stream verdict completed with auxiliary warnings event=%s target=%s warning_count=%d", target.ID, target.Target, len(auxiliaryFailures))
	}
	return nil
}

func (w *arvisStreamVerdictWorker) markTarget(ctx context.Context, eventID, status, lastError string) {
	processedAt := any(nil)
	if status == "completed" || status == "insufficient_evidence" {
		processedAt = time.Now().UTC()
	}
	_, _ = w.db.ExecContext(ctx, `
		UPDATE arvis_stream_processing
		SET status=$2,last_error=$3,processed_at=$4,updated_at=now()
		WHERE stream_event_id=$1::uuid
	`, eventID, status, compactArvisWorkerError(lastError), processedAt)
}

func arvisStreamVerdictInterval() time.Duration {
	seconds := 12
	if raw := strings.TrimSpace(os.Getenv("ARVIS_STREAM_VERDICT_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 5 && n <= 300 {
			seconds = n
		}
	}
	return time.Duration(seconds) * time.Second
}

func arvisStreamVerdictBatchSize() int {
	batchSize := 20
	if raw := strings.TrimSpace(os.Getenv("ARVIS_STREAM_VERDICT_BATCH_SIZE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 5 && n <= 100 {
			batchSize = n
		}
	}
	return batchSize
}

func arvisStreamVerdictConcurrency() int {
	concurrency := 4
	if raw := strings.TrimSpace(os.Getenv("ARVIS_STREAM_VERDICT_CONCURRENCY")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= 12 {
			concurrency = n
		}
	}
	return concurrency
}

func cloneArvisSignals(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+5)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func formatArvisWorkerError(err error) string {
	if err == nil {
		return ""
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		parts := []string{"sqlstate=" + string(pqErr.Code)}
		if strings.TrimSpace(pqErr.Constraint) != "" {
			parts = append(parts, "constraint="+pqErr.Constraint)
		}
		if strings.TrimSpace(pqErr.Message) != "" {
			parts = append(parts, "message="+pqErr.Message)
		}
		return compactArvisWorkerError(strings.Join(parts, " "))
	}
	return compactArvisWorkerError(err.Error())
}

func compactArvisWorkerError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 500 {
		return value[:500]
	}
	return value
}

func isArvisUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate key") || strings.Contains(text, "23505") || strings.Contains(text, "unique constraint")
}

func isUndefinedTableError(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "42P01" {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "does not exist") || strings.Contains(text, "undefined table")
}
