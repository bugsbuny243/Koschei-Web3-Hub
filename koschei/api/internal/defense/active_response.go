package defense

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ActiveResponse is a strictly non-executing planning boundary. No external
// network, cloud, Web3, identity, or hardware provider is called by this file.
// A future execution adapter MUST implement independent ownership, approval,
// provider-attestation, and postcondition checks before any side effect.
const ActiveResponseSchemaVersion = "koschei-active-response-plan-v1"

const (
	ActiveResponseShadowProposed = "shadow_proposed"
	ActiveResponseDenied         = "denied"
)

// ActiveResponseSpecification enumerates the only actions the planner can
// propose. The capability is descriptive, not an executable command.
type ActiveResponseSpecification struct {
	Action             string `json:"action"`
	Domain             string `json:"domain"`
	TargetKind         string `json:"target_kind"`
	ProviderCapability string `json:"provider_capability"`
	ExpectedEffect     string `json:"expected_effect"`
	MaxDurationSeconds int    `json:"max_duration_seconds"`
	HighImpact         bool   `json:"high_impact"`
}

var activeResponseSpecifications = []ActiveResponseSpecification{
	{"isolate_workload", "soar_xdr", "workload", "workload_quarantine", "isolated", 900, false},
	{"terminate_session", "soar_xdr", "session", "identity_session_revocation", "revoked", 0, false},
	{"revoke_api_key", "soar_xdr", "api_key", "identity_api_key_revocation", "revoked", 0, false},
	{"isolate_segment", "zero_trust", "network_segment", "microsegmentation_isolation", "isolated", 300, true},
	{"bgp_blackhole", "network_ddos", "protected_prefix", "routing_blackhole", "blackholed", 300, true},
	{"ddos_scrubbing", "network_ddos", "protected_prefix", "scrubbing_diversion", "scrubbing_enabled", 900, true},
	{"pause_contract", "web3", "smart_contract", "contract_emergency_pause", "paused", 0, true},
	{"block_transaction", "web3", "transaction_policy", "pre_sign_transaction_guard", "blocked_before_signing", 0, false},
	{"air_gap", "hardware", "hardware_link", "hardware_network_disconnection", "disconnected", 0, true},
	{"lock_hsm", "hardware", "hsm_key", "hardware_signer_lock", "signing_disabled", 0, true},
}

// ActiveResponseCatalog returns a copy to prevent callers from altering the
// authoritative set of supported defensive actions.
func ActiveResponseCatalog() []ActiveResponseSpecification {
	catalog := append([]ActiveResponseSpecification(nil), activeResponseSpecifications...)
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Action < catalog[j].Action })
	return catalog
}

// ProtectedAsset MUST originate from a trusted, tenant-bound asset inventory,
// not a user-supplied JSON body or model-generated recommendation.
type ProtectedAsset struct {
	TenantID           string `json:"tenant_id"`
	TargetID           string `json:"target_id"`
	Kind               string `json:"kind"`
	Managed            bool   `json:"managed"`
	ControlPlane       bool   `json:"control_plane"`
	ProductionCritical bool   `json:"production_critical"`
}

// ActiveResponsePlan never asserts that a provider changed the real world.
// ExecutionPerformed is always false, even for an authorized proposal.
type ActiveResponsePlan struct {
	SchemaVersion         string         `json:"schema_version"`
	ActionID              string         `json:"action_id"`
	TenantID              string         `json:"tenant_id"`
	TargetID              string         `json:"target_id"`
	Action                string         `json:"action"`
	Status                string         `json:"status"`
	Policy                PolicyDecision `json:"policy"`
	RequiresHumanApproval bool           `json:"requires_human_approval"`
	ExecutionPerformed    bool           `json:"execution_performed"`
	EffectAgreement       string         `json:"effect_agreement"`
	ExpectedEffect        string         `json:"expected_effect"`
	ProviderCapability    string         `json:"provider_capability"`
	MaxDurationSeconds    int            `json:"max_duration_seconds"`
	Reason                string         `json:"reason"`
}

// ProposeActiveResponse is additive and side-effect free. It reuses existing
// evidence and policy contracts without extending their wire format.
// Known evidence and protected assets MUST be resolved server-side by trusted
// services, rather than accepted as caller-authorized input.
func ProposeActiveResponse(policy DefensivePolicy, finding SecurityFinding, req DefensiveActionRequest, knownEvidence []string, asset ProtectedAsset) (ActiveResponsePlan, error) {
	plan := ActiveResponsePlan{
		SchemaVersion:      ActiveResponseSchemaVersion,
		ActionID:           req.ActionID,
		TenantID:           req.TenantID,
		TargetID:           req.TargetID,
		Action:             req.Action,
		Status:             ActiveResponseDenied,
		ExecutionPerformed: false,
		EffectAgreement:    ControlEffectNotExercised,
	}
	spec, ok := activeResponseSpecification(req.Action)
	if !ok {
		return plan, fmt.Errorf("unsupported defensive action %q", req.Action)
	}
	plan.ExpectedEffect = spec.ExpectedEffect
	plan.ProviderCapability = spec.ProviderCapability
	plan.MaxDurationSeconds = spec.MaxDurationSeconds

	if strings.TrimSpace(req.ActionID) == "" || strings.TrimSpace(req.TargetID) == "" || strings.TrimSpace(req.TenantID) == "" {
		return plan, errors.New("action_id, tenant_id and target_id are required")
	}
	if math.IsNaN(finding.Confidence) || math.IsInf(finding.Confidence, 0) {
		return plan, errors.New("confidence must be finite")
	}
	if err := ValidateFinding(finding, knownEvidence); err != nil {
		return plan, fmt.Errorf("unverified finding: %w", err)
	}
	if req.FindingID != finding.FindingID || req.Action != finding.DefensiveAction {
		return plan, errors.New("action is not bound to the verified finding")
	}
	if err := requireExactEvidence(req.EvidenceIDs, finding.EvidenceIDs); err != nil {
		return plan, err
	}
	minConfidence := 0.85
	if spec.HighImpact {
		minConfidence = 0.95
	}
	if spec.HighImpact && finding.Severity != "high" && finding.Severity != "critical" {
		plan.Reason = "high-impact action requires a high or critical verified finding"
		return plan, nil
	}
	if finding.Confidence < minConfidence {
		plan.Reason = "finding confidence is below the action's threshold"
		return plan, nil
	}
	if !asset.Managed || asset.ControlPlane || strings.TrimSpace(asset.TenantID) != strings.TrimSpace(req.TenantID) ||
		strings.TrimSpace(asset.TargetID) != strings.TrimSpace(req.TargetID) || asset.Kind != spec.TargetKind {
		plan.Reason = "asset is not a managed, matching, non-control-plane protected target"
		return plan, nil
	}
	plan.Policy = EvaluateDefensivePolicy(policy, req)
	if !plan.Policy.Allowed {
		plan.Reason = plan.Policy.Reason
		return plan, nil
	}
	plan.RequiresHumanApproval = plan.Policy.RequiresHumanApproval || spec.HighImpact || asset.ProductionCritical
	plan.Status = ActiveResponseShadowProposed
	plan.Reason = "policy-checked shadow proposal only; no provider effect has been attempted"
	return plan, nil
}

func activeResponseSpecification(action string) (ActiveResponseSpecification, bool) {
	for _, item := range activeResponseSpecifications {
		if item.Action == action {
			return item, true
		}
	}
	return ActiveResponseSpecification{}, false
}

func requireExactEvidence(actual, expected []string) error {
	if len(actual) == 0 || len(actual) != len(expected) {
		return errors.New("action evidence must match verified finding evidence")
	}
	allowed := stringSet(expected)
	seen := make(map[string]struct{}, len(actual))
	for _, id := range actual {
		if _, duplicate := seen[id]; duplicate {
			return errors.New("duplicate action evidence id")
		}
		seen[id] = struct{}{}
		if _, ok := allowed[id]; !ok {
			return errors.New("action evidence does not belong to verified finding")
		}
	}
	return nil
}
