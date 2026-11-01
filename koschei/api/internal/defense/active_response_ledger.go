package defense

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// This opt-in audit repository is not registered with an API route or worker.
// It stores SHADOW REVIEW metadata only. It cannot issue an execution permit,
// dispatch a provider command or claim that a security action took effect.
const ActiveResponseLedgerSchemaVersion = "koschei-active-response-shadow-ledger-v1"

const (
	ActiveResponseLedgerRecorded  = "shadow_recorded"
	ActiveResponseLedgerDuplicate = "shadow_duplicate"
)

type ActiveResponseLedgerEntry struct {
	SchemaVersion      string    `json:"schema_version"`
	TenantID           string    `json:"tenant_id"`
	ActionID           string    `json:"action_id"`
	TargetID           string    `json:"target_id"`
	ProviderID         string    `json:"provider_id"`
	PlanDigest         string    `json:"plan_digest"`
	PreviousHash       string    `json:"previous_hash"`
	ChainHash          string    `json:"chain_hash"`
	RecordedAt         time.Time `json:"recorded_at"`
	ExecutionPerformed bool      `json:"execution_performed"`
}

type ActiveResponseLedgerResult struct {
	Status             string `json:"status"`
	ChainHash          string `json:"chain_hash"`
	PlanDigest         string `json:"plan_digest"`
	ExecutionPerformed bool   `json:"execution_performed"`
}

func validShadowLedgerID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 128 &&
		!strings.ContainsAny(value, "\x00\n\r\t")
}

// ValidateShadowLedgerPreflight restricts the ledger to coherent offline
// readiness results. These objects are NOT trusted authentication inputs;
// callers must independently retrieve and verify them server-side.
func ValidateShadowLedgerPreflight(plan ActiveResponsePlan, review ActiveResponseReview, recovery ActiveResponseRecoveryAssessment) error {
	if plan.SchemaVersion != ActiveResponseSchemaVersion ||
		plan.Status != ActiveResponseShadowProposed ||
		plan.ExecutionPerformed || plan.EffectAgreement != ControlEffectNotExercised ||
		!validShadowLedgerID(plan.TenantID) ||
		!validShadowLedgerID(plan.ActionID) ||
		!validShadowLedgerID(plan.TargetID) {
		return errors.New("invalid or non-shadow action plan")
	}
	if review.SchemaVersion != ActiveResponseReviewSchemaVersion ||
		review.Status != ActiveResponseReviewReady ||
		review.ExecutionPerformed ||
		review.ActionID != plan.ActionID ||
		!validShadowLedgerID(review.ProviderID) ||
		!validActiveResponseSHA256(review.PlanDigest) ||
		review.VerifiedApprovals < review.RequiredApprovals {
		return errors.New("offline review is not ready")
	}
	if recovery.SchemaVersion != ActiveResponseRecoverySchemaVersion ||
		(recovery.Status != ActiveResponseRecoveryReview && recovery.Status != ActiveResponseRecoveryManual) ||
		recovery.ExecutionPerformed || recovery.RollbackPerformed ||
		recovery.ActionID != plan.ActionID ||
		recovery.ProviderID != review.ProviderID ||
		recovery.PlanDigest != review.PlanDigest ||
		len(recovery.Requirements) != 0 {
		return errors.New("offline recovery preflight is not ready")
	}
	if spec, ok := activeResponseSpecification(plan.Action); !ok ||
		plan.ProviderCapability != spec.ProviderCapability ||
		plan.ExpectedEffect != spec.ExpectedEffect ||
		plan.MaxDurationSeconds != spec.MaxDurationSeconds {
		return errors.New("action capability is not in the approved catalog")
	}
	return nil
}

// BuildShadowLedgerEntry is a deterministic, content-bound audit-chain helper.
// A hash chain is only tamper-evident when append-only DB permissions and
// independently stored checkpoints protect it; this module does not claim
// cryptographic immutability against database administrators.
func BuildShadowLedgerEntry(
	plan ActiveResponsePlan,
	review ActiveResponseReview,
	recovery ActiveResponseRecoveryAssessment,
	previousHash string,
	now time.Time,
) (ActiveResponseLedgerEntry, error) {
	if err := ValidateShadowLedgerPreflight(plan, review, recovery); err != nil {
		return ActiveResponseLedgerEntry{}, err
	}
	if now.IsZero() {
		return ActiveResponseLedgerEntry{}, errors.New("trusted recording timestamp is required")
	}
	if previousHash == "" {
		previousHash = "sha256:" + strings.Repeat("0", 64)
	}
	if !validActiveResponseSHA256(previousHash) {
		return ActiveResponseLedgerEntry{}, errors.New("invalid previous chain hash")
	}
	entry := ActiveResponseLedgerEntry{
		SchemaVersion:      ActiveResponseLedgerSchemaVersion,
		TenantID:           plan.TenantID,
		ActionID:           plan.ActionID,
		TargetID:           plan.TargetID,
		ProviderID:         review.ProviderID,
		PlanDigest:         review.PlanDigest,
		PreviousHash:       previousHash,
		RecordedAt:         now.UTC(),
		ExecutionPerformed: false,
	}
	chainHash, err := shadowLedgerEntryHash(entry)
	if err != nil {
		return ActiveResponseLedgerEntry{}, fmt.Errorf("hash shadow ledger entry: %w", err)
	}
	entry.ChainHash = chainHash
	return entry, nil
}

// RecordShadowLedger persists the planned action exactly once per tenant and
// action ID. It requires the separate, manually installed staging SQL schema:
// docs/security-center/ACTIVE_RESPONSE_LEDGER_STAGING_V1.sql.
// It is NOT invoked by any production route, worker or start-up function.
func RecordShadowLedger(
	ctx context.Context,
	db *sql.DB,
	plan ActiveResponsePlan,
	review ActiveResponseReview,
	recovery ActiveResponseRecoveryAssessment,
	now time.Time,
) (ActiveResponseLedgerResult, error) {
	if db == nil {
		return ActiveResponseLedgerResult{}, errors.New("shadow ledger database unavailable")
	}
	if err := ValidateShadowLedgerPreflight(plan, review, recovery); err != nil {
		return ActiveResponseLedgerResult{}, err
	}
	if now.IsZero() {
		return ActiveResponseLedgerResult{}, errors.New("trusted recording timestamp is required")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ActiveResponseLedgerResult{}, fmt.Errorf("begin shadow ledger transaction: %w", err)
	}
	defer tx.Rollback()

	// This lock serializes appends for one tenant on PostgreSQL; a hash collision
	// only serializes extra tenants, and does not grant cross-tenant visibility.
	if _, err = tx.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1::text, 314159))`, plan.TenantID); err != nil {
		return ActiveResponseLedgerResult{}, fmt.Errorf("lock shadow ledger tenant: %w", err)
	}

	var savedTarget, savedProvider, savedDigest, savedChain string
	err = tx.QueryRowContext(ctx, `SELECT target_id, provider_id, plan_digest, chain_hash
		FROM koschei_active_response_shadow_plans
		WHERE tenant_id = $1 AND action_id = $2 FOR UPDATE`,
		plan.TenantID, plan.ActionID).Scan(&savedTarget, &savedProvider, &savedDigest, &savedChain)
	if err == nil {
		if savedTarget != plan.TargetID || savedProvider != review.ProviderID ||
			savedDigest != review.PlanDigest || !validActiveResponseSHA256(savedChain) {
			return ActiveResponseLedgerResult{}, errors.New("shadow ledger replay collision: action ID already has different contents")
		}
		if err = tx.Commit(); err != nil {
			return ActiveResponseLedgerResult{}, fmt.Errorf("commit duplicate shadow ledger read: %w", err)
		}
		return ActiveResponseLedgerResult{
			Status: ActiveResponseLedgerDuplicate, ChainHash: savedChain,
			PlanDigest: savedDigest, ExecutionPerformed: false,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ActiveResponseLedgerResult{}, fmt.Errorf("read shadow ledger action: %w", err)
	}

	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT chain_hash FROM koschei_active_response_shadow_audit
		WHERE tenant_id = $1 ORDER BY sequence_id DESC LIMIT 1`, plan.TenantID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ActiveResponseLedgerResult{}, fmt.Errorf("read shadow ledger chain tip: %w", err)
	}
	entry, err := BuildShadowLedgerEntry(plan, review, recovery, previousHash, now)
	if err != nil {
		return ActiveResponseLedgerResult{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO koschei_active_response_shadow_plans
		(tenant_id, action_id, target_id, provider_id, plan_digest, chain_hash, recorded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		entry.TenantID, entry.ActionID, entry.TargetID, entry.ProviderID,
		entry.PlanDigest, entry.ChainHash, entry.RecordedAt)
	if err != nil {
		return ActiveResponseLedgerResult{}, fmt.Errorf("persist shadow action: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO koschei_active_response_shadow_audit
		(tenant_id, action_id, plan_digest, previous_hash, chain_hash, recorded_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		entry.TenantID, entry.ActionID, entry.PlanDigest, entry.PreviousHash,
		entry.ChainHash, entry.RecordedAt)
	if err != nil {
		return ActiveResponseLedgerResult{}, fmt.Errorf("append shadow ledger audit: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return ActiveResponseLedgerResult{}, fmt.Errorf("commit shadow ledger append: %w", err)
	}
	return ActiveResponseLedgerResult{
		Status: ActiveResponseLedgerRecorded, ChainHash: entry.ChainHash,
		PlanDigest: entry.PlanDigest, ExecutionPerformed: false,
	}, nil
}
