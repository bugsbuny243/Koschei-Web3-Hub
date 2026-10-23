package services

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ArvisStreamCanonicalSource marks canonical investigation jobs that were
// opened for one claimed Pump/Raydium stream event. The job worker hands their
// actor/behavior evidence to PersistArvisStreamCanonicalJobFinal.
const ArvisStreamCanonicalSource = "arvis_stream"

const arvisStreamCanonicalJobsEnv = "ARVIS_STREAM_CANONICAL_JOBS_ENABLED"

// ArvisStreamCanonicalEvent is a claimed stream event awaiting its canonical
// investigation. It carries identity only, never evidence or authority.
type ArvisStreamCanonicalEvent struct {
	ID        string
	Target    string
	Signature string
	Network   string
}

// ArvisStreamCanonicalJobsReady reports whether stream events may be turned
// into canonical investigation jobs. It is off unless explicitly enabled, and
// it stays off while any dependency of the signed-final path is missing, so
// events are never claimed only to fail on configuration.
func ArvisStreamCanonicalJobsReady() (bool, string) {
	if !envBool(arvisStreamCanonicalJobsEnv) {
		return false, arvisStreamCanonicalJobsEnv + " is not enabled"
	}
	if strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")) == "" {
		return false, "SOLANA_RPC_URL is not configured for native source proof"
	}
	registry, configured, err := arvisStreamRecoveryTrustRegistry()
	if err != nil {
		return false, err.Error()
	}
	if !configured {
		return false, arvisStreamTrustedKeysEnv + " is not configured"
	}
	keyID, privateKey, signerConfigured, err := unifiedVerdictSignerFromEnv()
	if err != nil {
		return false, err.Error()
	}
	if !signerConfigured {
		return false, "unified verdict signer is not configured"
	}
	trusted, err := registry.Resolve(keyID, time.Now().UTC())
	if err != nil {
		return false, fmt.Sprintf("signer key %q is not in the trusted registry: %v", keyID, err)
	}
	public, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || !bytes.Equal(public, trusted) {
		return false, fmt.Sprintf("signer key %q does not match its trusted public key", keyID)
	}
	return true, ""
}

// ClaimArvisStreamCanonicalEvents claims up to limit pending stream events,
// at most one per mint, for canonical investigation. Events whose source
// program or network the signed-final path cannot accept are closed as
// insufficient evidence instead of being retried forever.
func ClaimArvisStreamCanonicalEvents(ctx context.Context, db *sql.DB, limit int) ([]ArvisStreamCanonicalEvent, error) {
	if limit <= 0 {
		return []ArvisStreamCanonicalEvent{}, nil
	}
	// Read a wider window so duplicate mints do not starve distinct ones.
	pending, err := arvisStreamPendingTargets(ctx, db, limit*5)
	if err != nil {
		return nil, err
	}
	out := []ArvisStreamCanonicalEvent{}
	seen := map[string]bool{}
	for _, target := range pending {
		if len(out) >= limit {
			break
		}
		if seen[target.Target] {
			continue
		}
		seen[target.Target] = true
		if !claimArvisStreamTarget(ctx, db, target) {
			continue
		}
		if target.Network != "solana-mainnet" || !arvisStreamSourceProgramPermitted(target.ModuleID, target.ProgramID) {
			unsupported := fmt.Errorf("%w: stream source program or network is not accepted for a signed final", errArvisStreamInsufficientEvidence)
			if _, err := MarkArvisStreamCanonicalOutcome(ctx, db, target.ID, unsupported); err != nil {
				return out, err
			}
			continue
		}
		out = append(out, ArvisStreamCanonicalEvent{
			ID: target.ID, Target: target.Target, Signature: target.Signature, Network: target.Network,
		})
	}
	return out, nil
}

// MarkArvisStreamCanonicalOutcome closes a claim whose canonical final was
// withheld. Missing actor/behavior evidence becomes insufficient_evidence;
// every other cause is a retryable operational failure. It can never mark a
// claim completed, and it leaves a claim backed by a signed final untouched.
func MarkArvisStreamCanonicalOutcome(ctx context.Context, db *sql.DB, eventID string, outcome error) (string, error) {
	status := "failed"
	if errors.Is(outcome, errArvisStreamInsufficientEvidence) {
		status = "insufficient_evidence"
	}
	message := ""
	if outcome != nil {
		message = formatArvisWorkerError(outcome)
	}
	if db == nil {
		return status, errors.New("ARVIS stream claim database unavailable")
	}
	if _, err := markArvisStreamNonFinalClaim(ctx, db, eventID, status, message); err != nil {
		return status, err
	}
	return status, nil
}

// ReleaseStaleArvisStreamCanonicalClaims fails processing claims older than
// olderThan that have no signed final, so a canonical job that died before
// reporting its outcome is retried within the normal attempt budget.
func ReleaseStaleArvisStreamCanonicalClaims(ctx context.Context, db *sql.DB, olderThan time.Duration, limit int) (int, error) {
	if db == nil {
		return 0, errors.New("ARVIS stream claim database unavailable")
	}
	if olderThan <= 0 || limit <= 0 {
		return 0, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT p.stream_event_id::text
		FROM arvis_stream_processing AS p
		WHERE p.status='processing'
		  AND p.updated_at < now() - make_interval(secs => $1)
		  AND NOT EXISTS (
		    SELECT 1 FROM arvis_stream_canonical_finals AS f
		    WHERE f.stream_event_id=p.stream_event_id
		  )
		ORDER BY p.updated_at
		LIMIT $2
	`, olderThan.Seconds(), limit)
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	released := 0
	for _, id := range ids {
		done, err := markArvisStreamNonFinalClaim(ctx, db, id, "failed", "canonical investigation did not report an outcome")
		if err != nil {
			return released, err
		}
		if done {
			released++
		}
	}
	return released, nil
}
