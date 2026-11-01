package defense

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const ActiveResponseReviewSchemaVersion = "koschei-active-response-review-v1"

const (
	ActiveResponseReviewDenied  = "denied"
	ActiveResponseReviewPending = "pending_approval"
	ActiveResponseReviewReady   = "ready_for_manual_integration"
)

// ActiveResponseProviderBinding is an inventory/configuration assertion only.
// It must come from a trusted, operator-managed registry. This is not a
// provider-side authority proof and never authorizes a live side effect.
type ActiveResponseProviderBinding struct {
	ProviderID string `json:"provider_id"`
	TenantID   string `json:"tenant_id"`
	TargetID   string `json:"target_id"`
	Action     string `json:"action"`
	Capability string `json:"capability"`
	Enabled    bool   `json:"enabled"`
}

// ActiveResponseApproval represents an offline Ed25519 approval for a single
// content-addressed action plan. Approver keys must be provided from a trusted
// server-side registry, not from the untrusted request being approved.
type ActiveResponseApproval struct {
	ApproverID string    `json:"approver_id"`
	PlanDigest string    `json:"plan_digest"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Signature  []byte    `json:"signature"`
}

// ActiveResponseReview never grants live execution authority. A ready result
// means only that the supplied offline review evidence passed local checks.
type ActiveResponseReview struct {
	SchemaVersion      string `json:"schema_version"`
	Status             string `json:"status"`
	ActionID           string `json:"action_id"`
	PlanDigest         string `json:"plan_digest"`
	ProviderID         string `json:"provider_id"`
	RequiredApprovals  int    `json:"required_approvals"`
	VerifiedApprovals  int    `json:"verified_approvals"`
	ExecutionPerformed bool   `json:"execution_performed"`
	Reason             string `json:"reason"`
}

// ActiveResponsePlanDigest binds the exact proposal to the trusted finding,
// managed asset, requested parameters and existing defensive policy. It is not
// an authorization token and cannot be used as an idempotency or replay store.
func ActiveResponsePlanDigest(policy DefensivePolicy, finding SecurityFinding, req DefensiveActionRequest, asset ProtectedAsset, plan ActiveResponsePlan) (string, error) {
	if plan.Status != ActiveResponseShadowProposed || plan.ExecutionPerformed || plan.EffectAgreement != ControlEffectNotExercised {
		return "", errors.New("only unexecuted shadow plans can be reviewed")
	}
	payload := struct {
		SchemaVersion string                 `json:"schema_version"`
		Policy        DefensivePolicy        `json:"policy"`
		Finding       SecurityFinding        `json:"finding"`
		Request       DefensiveActionRequest `json:"request"`
		Asset         ProtectedAsset         `json:"asset"`
		Plan          ActiveResponsePlan     `json:"plan"`
	}{
		ActiveResponseReviewSchemaVersion, policy, finding, req, asset, plan,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode active response review: %w", err)
	}
	digest := sha256.Sum256(append([]byte("koschei-active-response-review-v1\n"), encoded...))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// ActiveResponseApprovalSigningPayload returns the exact bytes each approver
// must sign. No caller-provided free-form message is interpreted as a command.
func ActiveResponseApprovalSigningPayload(a ActiveResponseApproval) ([]byte, error) {
	if strings.TrimSpace(a.ApproverID) == "" || !strings.HasPrefix(a.PlanDigest, "sha256:") || len(a.PlanDigest) != 71 {
		return nil, errors.New("invalid approver or action plan digest")
	}
	rawDigest, err := hex.DecodeString(strings.TrimPrefix(a.PlanDigest, "sha256:"))
	if err != nil || len(rawDigest) != sha256.Size {
		return nil, errors.New("action plan digest must be a SHA-256 hex value")
	}
	if a.IssuedAt.IsZero() || a.ExpiresAt.IsZero() || !a.ExpiresAt.After(a.IssuedAt) ||
		a.ExpiresAt.Sub(a.IssuedAt) > 5*time.Minute {
		return nil, errors.New("approval validity must be at most five minutes")
	}
	return json.Marshal(struct {
		SchemaVersion string    `json:"schema_version"`
		ApproverID    string    `json:"approver_id"`
		PlanDigest    string    `json:"plan_digest"`
		IssuedAt      time.Time `json:"issued_at"`
		ExpiresAt     time.Time `json:"expires_at"`
	}{
		ActiveResponseReviewSchemaVersion, a.ApproverID, a.PlanDigest, a.IssuedAt.UTC(), a.ExpiresAt.UTC(),
	})
}

// ReviewActiveResponse is a side-effect-free trust boundary. It recomputes the
// original plan, verifies registry binding and optional offline signatures,
// and returns a review status. It deliberately has no provider executor.
func ReviewActiveResponse(
	policy DefensivePolicy,
	finding SecurityFinding,
	req DefensiveActionRequest,
	knownEvidence []string,
	asset ProtectedAsset,
	plan ActiveResponsePlan,
	provider ActiveResponseProviderBinding,
	approvals []ActiveResponseApproval,
	trustedApprovers map[string]ed25519.PublicKey,
	now time.Time,
) ActiveResponseReview {
	out := ActiveResponseReview{
		SchemaVersion:      ActiveResponseReviewSchemaVersion,
		Status:             ActiveResponseReviewDenied,
		ActionID:           req.ActionID,
		ProviderID:         provider.ProviderID,
		ExecutionPerformed: false,
	}
	if now.IsZero() {
		out.Reason = "explicit trusted evaluation time is required"
		return out
	}
	recomputed, err := ProposeActiveResponse(policy, finding, req, knownEvidence, asset)
	if err != nil || !reflect.DeepEqual(recomputed, plan) || plan.Status != ActiveResponseShadowProposed {
		out.Reason = "proposal does not match verified inputs"
		return out
	}
	digest, err := ActiveResponsePlanDigest(policy, finding, req, asset, plan)
	if err != nil {
		out.Reason = "proposal cannot be fingerprinted"
		return out
	}
	out.PlanDigest = digest
	if !provider.Enabled || strings.TrimSpace(provider.ProviderID) == "" ||
		provider.TenantID != req.TenantID || provider.TargetID != req.TargetID ||
		provider.Action != req.Action || provider.Capability != plan.ProviderCapability {
		out.Reason = "provider capability is not bound to the managed asset and action"
		return out
	}
	spec, ok := activeResponseSpecification(req.Action)
	if !ok {
		out.Reason = "unknown action"
		return out
	}
	if spec.HighImpact {
		out.RequiredApprovals = 2
	} else if plan.RequiresHumanApproval {
		out.RequiredApprovals = 1
	}
	seen := make(map[string]struct{}, len(approvals))
	for _, approval := range approvals {
		if _, duplicate := seen[approval.ApproverID]; duplicate {
			out.Reason = "duplicate human approver"
			return out
		}
		seen[approval.ApproverID] = struct{}{}
		key, trusted := trustedApprovers[approval.ApproverID]
		if !trusted || len(key) != ed25519.PublicKeySize || approval.PlanDigest != digest ||
			now.Before(approval.IssuedAt) || !now.Before(approval.ExpiresAt) {
			out.Reason = "untrusted, mismatched or expired approval"
			return out
		}
		bytesToVerify, err := ActiveResponseApprovalSigningPayload(approval)
		if err != nil || !ed25519.Verify(key, bytesToVerify, approval.Signature) {
			out.Reason = "invalid signed approval"
			return out
		}
		out.VerifiedApprovals++
	}
	if out.VerifiedApprovals < out.RequiredApprovals {
		out.Status = ActiveResponseReviewPending
		out.Reason = "independent signed human approvals are still required"
		return out
	}
	out.Status = ActiveResponseReviewReady
	out.Reason = "offline review passed; provider execution and effect verification remain disabled"
	return out
}
