package defense

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// TrustedShadowLedgerInputs must be constructed from an authenticated,
// tenant-bound operator registry and evidence store. Untrusted HTTP request
// bodies must never populate policy, asset ownership, provider binding,
// known evidence or approver trust anchors.
type TrustedShadowLedgerInputs struct {
	Policy           DefensivePolicy
	Finding          SecurityFinding
	Request          DefensiveActionRequest
	KnownEvidence    []string
	Asset            ProtectedAsset
	Provider         ActiveResponseProviderBinding
	Approvals        []ActiveResponseApproval
	TrustedApprovers map[string]ed25519.PublicKey
	Recovery         ActiveResponseRecoveryCapability
}

type VerifiedShadowLedgerPreflight struct {
	Plan     ActiveResponsePlan
	Review   ActiveResponseReview
	Recovery ActiveResponseRecoveryAssessment
}

// PrepareVerifiedShadowLedger recomputes the complete, offline trust chain.
// A caller cannot claim success by submitting a fabricated "ready" status.
// This function neither creates an execution permit nor changes external state.
func PrepareVerifiedShadowLedger(input TrustedShadowLedgerInputs, now time.Time) (VerifiedShadowLedgerPreflight, error) {
	if now.IsZero() {
		return VerifiedShadowLedgerPreflight{}, errors.New("trusted evaluation time is required")
	}
	plan, err := ProposeActiveResponse(
		input.Policy, input.Finding, input.Request, input.KnownEvidence, input.Asset,
	)
	if err != nil {
		return VerifiedShadowLedgerPreflight{}, fmt.Errorf("plan rejected: %w", err)
	}
	if plan.Status != ActiveResponseShadowProposed {
		return VerifiedShadowLedgerPreflight{}, errors.New("policy did not authorize a shadow proposal")
	}
	review := ReviewActiveResponse(
		input.Policy, input.Finding, input.Request, input.KnownEvidence, input.Asset,
		plan, input.Provider, input.Approvals, input.TrustedApprovers, now,
	)
	if review.Status != ActiveResponseReviewReady {
		return VerifiedShadowLedgerPreflight{}, errors.New("signed offline review is not complete")
	}
	digest, err := ActiveResponsePlanDigest(input.Policy, input.Finding, input.Request, input.Asset, plan)
	if err != nil || digest != review.PlanDigest {
		return VerifiedShadowLedgerPreflight{}, errors.New("verified review digest does not match source inputs")
	}
	recovery := AssessActiveResponseRecovery(plan, review, input.Recovery, now)
	if err := ValidateShadowLedgerPreflight(plan, review, recovery); err != nil {
		return VerifiedShadowLedgerPreflight{}, fmt.Errorf("recovery or audit preflight rejected: %w", err)
	}
	return VerifiedShadowLedgerPreflight{Plan: plan, Review: review, Recovery: recovery}, nil
}

// RecordVerifiedShadowLedger performs a verified offline preflight, then
// delegates to the optional staging-only PostgreSQL ledger. It never calls a
// cloud, network, smart-contract, identity or HSM execution adapter.
// The separate staging schema must already exist; this method is not routed.
func RecordVerifiedShadowLedger(
	ctx context.Context,
	db *sql.DB,
	input TrustedShadowLedgerInputs,
	now time.Time,
) (ActiveResponseLedgerResult, error) {
	if db == nil {
		return ActiveResponseLedgerResult{}, errors.New("shadow ledger database unavailable")
	}
	preflight, err := PrepareVerifiedShadowLedger(input, now)
	if err != nil {
		return ActiveResponseLedgerResult{}, err
	}
	return retryShadowLedgerSerialization(ctx, func() (ActiveResponseLedgerResult, error) {
		return recordShadowLedger(ctx, db, preflight.Plan, preflight.Review, preflight.Recovery, now)
	})
}

// retryShadowLedgerSerialization retries only PostgreSQL 40001 errors, never
// validation, uniqueness, permission, connection or ambiguous commit errors.
// Each invocation of attempt creates its own transaction; no live provider
// action is reachable. Context cancellation is always terminal.
func retryShadowLedgerSerialization(
	ctx context.Context,
	attempt func() (ActiveResponseLedgerResult, error),
) (ActiveResponseLedgerResult, error) {
	const maxAttempts = 5
	for n := 0; n < maxAttempts; n++ {
		if err := ctx.Err(); err != nil {
			return ActiveResponseLedgerResult{}, err
		}
		result, err := attempt()
		if err == nil {
			return result, nil
		}
		var pgErr *pq.Error
		if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
			return ActiveResponseLedgerResult{}, err
		}
		if n == maxAttempts-1 {
			return ActiveResponseLedgerResult{}, fmt.Errorf("shadow ledger serialization retries exhausted: %w", err)
		}
	}
	return ActiveResponseLedgerResult{}, errors.New("shadow ledger retry guard rejected")
}
