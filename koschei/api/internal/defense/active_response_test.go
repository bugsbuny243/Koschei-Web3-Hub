package defense

import "testing"

func activeResponseFixture(action, kind string) (DefensivePolicy, SecurityFinding, DefensiveActionRequest, ProtectedAsset) {
	policy := DefensivePolicy{TenantID: "tenant-a", AllowedActions: []string{action}, AllowedTargets: []string{"asset-a"}}
	finding := SecurityFinding{FindingID: "finding-a", Severity: "critical", Confidence: 0.99, EvidenceIDs: []string{"signal-a", "signal-b"}, DefensiveAction: action}
	req := DefensiveActionRequest{ActionID: "action-a", TenantID: "tenant-a", TargetID: "asset-a", Action: action, FindingID: "finding-a", EvidenceIDs: []string{"signal-b", "signal-a"}}
	asset := ProtectedAsset{TenantID: "tenant-a", TargetID: "asset-a", Kind: kind, Managed: true}
	return policy, finding, req, asset
}

func TestActiveResponseCatalogCoversFiveDomains(t *testing.T) {
	catalog := ActiveResponseCatalog()
	domains := map[string]bool{}
	for _, item := range catalog {
		domains[item.Domain] = true
		if item.Action == "" || item.TargetKind == "" || item.ProviderCapability == "" {
			t.Fatalf("incomplete catalog action: %+v", item)
		}
	}
	for _, name := range []string{"soar_xdr", "zero_trust", "network_ddos", "web3", "hardware"} {
		if !domains[name] {
			t.Errorf("missing domain: %s", name)
		}
	}
	catalog[0].Action = "tamper"
	if next := ActiveResponseCatalog(); next[0].Action == "tamper" {
		t.Fatal("catalog leaked mutable state")
	}
}

func TestActiveResponsePlannerDoesNotExecute(t *testing.T) {
	for _, spec := range ActiveResponseCatalog() {
		t.Run(spec.Action, func(t *testing.T) {
			policy, finding, req, asset := activeResponseFixture(spec.Action, spec.TargetKind)
			plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Status != ActiveResponseShadowProposed || plan.ExecutionPerformed || plan.EffectAgreement != ControlEffectNotExercised {
				t.Fatalf("expected shadow-only proposal: %+v", plan)
			}
			if spec.HighImpact && !plan.RequiresHumanApproval {
				t.Fatal("high-impact action bypassed human approval")
			}
		})
	}
}

func TestActiveResponseRejectsCrossTenantAndUnmanagedTargets(t *testing.T) {
	policy, finding, req, asset := activeResponseFixture("isolate_workload", "workload")
	asset.TenantID = "tenant-b"
	plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("cross-tenant: %+v, %v", plan, err)
	}
	asset.TenantID = "tenant-a"
	asset.Managed = false
	plan, err = ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("unmanaged: %+v, %v", plan, err)
	}
	asset.Managed = true
	asset.ControlPlane = true
	plan, err = ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("control-plane: %+v, %v", plan, err)
	}
	asset.ControlPlane = false
	asset.Kind = "smart_contract"
	plan, err = ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("wrong target kind: %+v, %v", plan, err)
	}
}

func TestActiveResponseRejectsUntrustedEvidenceAndUnknownActions(t *testing.T) {
	policy, finding, req, asset := activeResponseFixture("revoke_api_key", "api_key")
	req.EvidenceIDs = []string{"signal-b", "forged"}
	if _, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset); err == nil {
		t.Fatal("forged evidence accepted")
	}
	req.EvidenceIDs = []string{"signal-a", "signal-b"}
	if _, err := ProposeActiveResponse(policy, finding, req, []string{"signal-a"}, asset); err == nil {
		t.Fatal("unknown evidence accepted")
	}
	req.Action = "arbitrary_shell_command"
	if _, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset); err == nil {
		t.Fatal("arbitrary action accepted")
	}
}

func TestActiveResponseRequiresExplicitPolicyAndConfidence(t *testing.T) {
	policy, finding, req, asset := activeResponseFixture("bgp_blackhole", "protected_prefix")
	policy.AllowedActions = nil
	plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("missing action allowlist: %+v, %v", plan, err)
	}
	policy.AllowedActions = []string{req.Action}
	finding.Confidence = 0.91
	plan, err = ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("insufficient confidence: %+v, %v", plan, err)
	}
	finding.Confidence = 0.99
	asset.ProductionCritical = true
	plan, err = ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseShadowProposed || !plan.RequiresHumanApproval {
		t.Fatalf("critical target gate missing: %+v, %v", plan, err)
	}
}

func TestActiveResponsePreservesExistingHumanGate(t *testing.T) {
	policy, finding, req, asset := activeResponseFixture("terminate_session", "session")
	policy.HumanGateActions = []string{req.Action}
	plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || !plan.RequiresHumanApproval {
		t.Fatalf("policy human gate lost: %+v, %v", plan, err)
	}
}

func TestActiveResponseRejectsEmptyTargetAndLowSeverityHighImpact(t *testing.T) {
	policy, finding, req, asset := activeResponseFixture("pause_contract", "smart_contract")
	req.TargetID = ""
	if _, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset); err == nil {
		t.Fatal("empty target accepted")
	}
	req.TargetID = "asset-a"
	finding.Severity = "low"
	plan, err := ProposeActiveResponse(policy, finding, req, finding.EvidenceIDs, asset)
	if err != nil || plan.Status != ActiveResponseDenied {
		t.Fatalf("low-severity contract pause proposal accepted: %+v, %v", plan, err)
	}
}
