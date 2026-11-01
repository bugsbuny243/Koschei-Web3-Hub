package defense

import (
	"errors"
	"strings"
	"time"
)

// Recovery readiness is an offline prerequisite review, never a rollback
// instruction, execution credential, provider API invocation or live permit.
const ActiveResponseRecoverySchemaVersion = "koschei-active-response-recovery-v1"

const (
	ActiveResponseRecoveryBlocked = "blocked"
	ActiveResponseRecoveryReview  = "ready_for_staging_review"
	ActiveResponseRecoveryManual  = "manual_recovery_review"
)

// ActiveResponseRecoveryCapability is supplied by a trusted inventory of
// provider-specific operational recovery capabilities, not by a request body.
type ActiveResponseRecoveryCapability struct {
	ProviderID             string    `json:"provider_id"`
	TenantID               string    `json:"tenant_id"`
	TargetID               string    `json:"target_id"`
	Action                 string    `json:"action"`
	RunbookRef             string    `json:"runbook_ref"`
	RecoveryOwnerID        string    `json:"recovery_owner_id"`
	IndependentObserverID  string    `json:"independent_observer_id"`
	ManagementPathExcluded bool      `json:"management_path_excluded"`
	BreakGlassVerified     bool      `json:"break_glass_verified"`
	RollbackTestAttested   bool      `json:"rollback_test_attested"`
	RollbackTestedAt       time.Time `json:"rollback_tested_at"`
	ProviderTTLSeconds     int       `json:"provider_ttl_seconds"`
}

// ActiveResponseRecoveryAssessment reports only planning readiness.
// ExecutionPerformed and RollbackPerformed must remain false.
type ActiveResponseRecoveryAssessment struct {
	SchemaVersion      string   `json:"schema_version"`
	Status             string   `json:"status"`
	ActionID           string   `json:"action_id"`
	PlanDigest         string   `json:"plan_digest"`
	ProviderID         string   `json:"provider_id"`
	RecoveryMode       string   `json:"recovery_mode"`
	Requirements       []string `json:"requirements"`
	ExecutionPerformed bool     `json:"execution_performed"`
	RollbackPerformed  bool     `json:"rollback_performed"`
	Reason             string   `json:"reason"`
}

// ActiveResponseRecoveryMode prevents misleading auto-reversal promises.
// Real reversal of credentials, HSM, physical links and smart contracts is
// provider-specific and cannot be inferred from an action name.
func ActiveResponseRecoveryMode(action string) (string, error) {
	switch action {
	case "isolate_workload", "isolate_segment", "bgp_blackhole", "ddos_scrubbing":
		return "provider_verified_timeout_and_restore", nil
	case "terminate_session", "revoke_api_key", "pause_contract", "block_transaction", "air_gap", "lock_hsm":
		return "documented_manual_recovery", nil
	default:
		return "", errors.New("unsupported active response recovery action")
	}
}

// AssessActiveResponseRecovery is deterministic and side-effect free.
// The trusted registry and review inputs must originate from server-side
// sources. A successful staging review is not approval to execute any action.
func AssessActiveResponseRecovery(
	plan ActiveResponsePlan,
	review ActiveResponseReview,
	capability ActiveResponseRecoveryCapability,
	now time.Time,
) ActiveResponseRecoveryAssessment {
	out := ActiveResponseRecoveryAssessment{
		SchemaVersion: ActiveResponseRecoverySchemaVersion,
		Status:        ActiveResponseRecoveryBlocked,
		ActionID:      plan.ActionID,
		PlanDigest:    review.PlanDigest,
		ProviderID:    capability.ProviderID,
		Requirements:  []string{},
	}
	if now.IsZero() {
		out.Reason = "trusted evaluation time is required"
		return out
	}
	mode, err := ActiveResponseRecoveryMode(plan.Action)
	if err != nil {
		out.Reason = "unsupported recovery action"
		return out
	}
	out.RecoveryMode = mode
	if plan.SchemaVersion != ActiveResponseSchemaVersion ||
		plan.Status != ActiveResponseShadowProposed ||
		plan.ExecutionPerformed || plan.EffectAgreement != ControlEffectNotExercised ||
		review.SchemaVersion != ActiveResponseReviewSchemaVersion ||
		review.Status != ActiveResponseReviewReady ||
		review.ExecutionPerformed ||
		review.ActionID != plan.ActionID ||
		review.RequiredApprovals > review.VerifiedApprovals ||
		strings.TrimSpace(review.PlanDigest) == "" {
		out.Reason = "verified, non-executing review must precede recovery planning"
		return out
	}
	if !capability.ManagementPathExcluded || !capability.BreakGlassVerified {
		out.Requirements = append(out.Requirements, "protected management access and tested break-glass path")
	}
	if strings.TrimSpace(capability.ProviderID) == "" ||
		capability.ProviderID != review.ProviderID ||
		capability.TenantID != plan.TenantID ||
		capability.TargetID != plan.TargetID ||
		capability.Action != plan.Action {
		out.Requirements = append(out.Requirements, "trusted provider, action and target binding")
	}
	if strings.TrimSpace(capability.RunbookRef) == "" {
		out.Requirements = append(out.Requirements, "approved versioned recovery runbook")
	}
	if strings.TrimSpace(capability.RecoveryOwnerID) == "" ||
		strings.TrimSpace(capability.IndependentObserverID) == "" ||
		capability.RecoveryOwnerID == capability.IndependentObserverID {
		out.Requirements = append(out.Requirements, "separate recovery owner and independent observer")
	}
	if mode == "provider_verified_timeout_and_restore" {
		if plan.MaxDurationSeconds <= 0 || capability.ProviderTTLSeconds <= 0 ||
			capability.ProviderTTLSeconds > plan.MaxDurationSeconds {
			out.Requirements = append(out.Requirements, "bounded provider-enforced timeout within action limit")
		}
		if !capability.RollbackTestAttested || capability.RollbackTestedAt.IsZero() ||
			capability.RollbackTestedAt.After(now) ||
			now.Sub(capability.RollbackTestedAt) > 30*24*time.Hour {
			out.Requirements = append(out.Requirements, "recent independently attested rollback test")
		}
	} else if plan.MaxDurationSeconds != 0 {
		out.Requirements = append(out.Requirements, "manual recovery action must not advertise automatic rollback")
	}
	if len(out.Requirements) > 0 {
		out.Reason = "recovery preconditions incomplete; no live execution permitted"
		return out
	}
	if mode == "provider_verified_timeout_and_restore" {
		out.Status = ActiveResponseRecoveryReview
	} else {
		out.Status = ActiveResponseRecoveryManual
	}
	out.Reason = "offline recovery prerequisites reviewed; production execution remains disabled"
	return out
}
