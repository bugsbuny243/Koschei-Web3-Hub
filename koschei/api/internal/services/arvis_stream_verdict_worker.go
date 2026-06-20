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
	"time"
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
	go (&arvisStreamVerdictWorker{db: db, store: NewSecurityRadarStore(db), interval: arvisStreamVerdictInterval()}).start(ctx)
	return cancel
}

type arvisStreamVerdictWorker struct {
	db       *sql.DB
	store    *SecurityRadarStore
	interval time.Duration
}

func (w *arvisStreamVerdictWorker) start(ctx context.Context) {
	if w == nil || w.db == nil || w.store == nil {
		return
	}
	log.Printf("arvis stream verdict worker started interval=%s", w.interval)
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
	targets, err := w.pendingTargets(ctx, 5)
	if err != nil {
		if !isUndefinedTableError(err) {
			log.Printf("arvis stream verdict queue read failed: %v", err)
		}
		return
	}
	for _, target := range targets {
		if !w.claimTarget(ctx, target) {
			continue
		}
		err := w.processTarget(ctx, target)
		if errors.Is(err, errArvisStreamInsufficientEvidence) {
			w.markTarget(ctx, target.ID, "insufficient_evidence", SecurityRadarInsufficientEvidenceMessage)
			continue
		}
		if err != nil {
			w.markTarget(ctx, target.ID, "failed", err.Error())
			log.Printf("arvis stream verdict failed event=%s target=%s module=%s err=%v", target.ID, target.Target, target.ModuleID, err)
			continue
		}
		w.markTarget(ctx, target.ID, "completed", "")
	}
}

func (w *arvisStreamVerdictWorker) pendingTargets(ctx context.Context, limit int) ([]arvisStreamTarget, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	rows, err := w.db.QueryContext(ctx, `
		SELECT s.id::text,
		       COALESCE(s.target,''),
		       COALESCE(s.signature,''),
		       COALESCE(s.network,'solana-mainnet'),
		       COALESCE(s.slot,0),
		       COALESCE(s.program_id,''),
		       COALESCE(s.module_id,'')
		FROM security_radar_stream_events s
		LEFT JOIN arvis_stream_processing p ON p.stream_event_id=s.id
		WHERE COALESCE(s.target,'') <> ''
		  AND COALESCE(s.target_type,'')='token'
		  AND COALESCE(s.module_id,'') IN ($2,$3)
		  AND (
			p.stream_event_id IS NULL
			OR (p.status='failed' AND p.attempts < 3 AND p.updated_at < now() - interval '30 seconds')
		  )
		ORDER BY s.created_at ASC
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
		WHERE arvis_stream_processing.status='failed' AND arvis_stream_processing.attempts < 3
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
	for _, arm := range arms {
		if !SecurityRadarVerdictHasVerifiedEvidence(arm) {
			continue
		}
		alreadySaved, err := w.streamArmAlreadySaved(ctx, target.ID, arm.ModuleID)
		if err != nil {
			return fmt.Errorf("check existing arm verdict %s: %w", arm.ModuleID, err)
		}
		if alreadySaved {
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
			return fmt.Errorf("insert arm event %s: %w", arm.ModuleID, err)
		}
		if _, err := w.store.InsertVerdict(ctx, SecurityRadarVerdictRecord{
			EventID: eventID, ModuleID: arm.ModuleID, Target: arm.Target, TargetType: "token", Network: arm.Network,
			Grade: arm.Grade, RiskIndex: arm.RiskIndex, RiskLevel: arm.RiskLevel, Verdict: arm.Verdict,
			Recommendation: arm.Recommendation, Evidence: arm.Evidence, Signals: signals,
			RuleVersion: arm.RuleVersion, Signed: arm.Signed, Signature: arm.Signature,
			Source: "arvis_stream", EventType: "arvis_stream_verdict", Provider: provider,
		}); err != nil {
			if isArvisUniqueViolation(err) {
				continue
			}
			return fmt.Errorf("insert arm verdict %s: %w", arm.ModuleID, err)
		}
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

func cloneArvisSignals(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+5)
	for key, value := range in {
		out[key] = value
	}
	return out
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
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate key") || strings.Contains(text, "23505") || strings.Contains(text, "unique constraint")
}

func isUndefinedTableError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "does not exist") || strings.Contains(text, "undefined table")
}
