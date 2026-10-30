package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func TestCustomerInvestigationStatusRequiresCryptographicallyVerifiedLiveVerdict(t *testing.T) {
	configureHandlerVerdictTestSigner(t)
	valid := services.FinalizeUnifiedRadarVerdictContractForNetwork("MintReady", "solana-mainnet", services.UnifiedRadarVerdict{
		RulesetVersion: services.UnifiedRadarRulesetVersion,
		ActorRuleset: services.ActorDefenseRulesetVersion,
		TriggeredRules: []services.ActorDefenseRuleHit{{
			RuleID: services.ActorRuleCompoundCreatorReuse,
			Tier: "compounding", EvidenceStatus: "verified", Summary: "verified actor evidence",
		}},
	})
	if !valid.Signed || valid.Signature == "" {
		t.Fatalf("test signed-final fixture unavailable: %#v", valid)
	}
	tampered := valid
	tampered.TriggeredRules = append([]services.ActorDefenseRuleHit{}, valid.TriggeredRules...)
	tampered.TriggeredRules[0].Summary = "mutated after signing"
	forged := valid
	forged.Signature = "fake-signature"
	wrongTarget := valid
	wrongTarget.Target = "DifferentMint"
	tests := []struct {
		name  string
		final services.UnifiedRadarVerdict
		live  bool
		want  string
	}{
		{name: "unsigned live evidence remains pending", final: services.UnifiedRadarVerdict{Grade: "-", Signed: false}, live: true, want: "evidence_pending"},
		{name: "signed flag without signature is not proof", final: services.UnifiedRadarVerdict{Grade: "B", Signed: true}, live: true, want: "evidence_pending"},
		{name: "signed result without live evidence remains pending", final: valid, live: false, want: "evidence_pending"},
		{name: "cryptographically valid live verdict is ready", final: valid, live: true, want: "ready"},
		{name: "altered evidence binding is rejected", final: tampered, live: true, want: "evidence_pending"},
		{name: "forged signature is rejected", final: forged, live: true, want: "evidence_pending"},
		{name: "changed target cannot reuse signature", final: wrongTarget, live: true, want: "evidence_pending"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := customerInvestigationStatus(tc.final, tc.live); got != tc.want {
				t.Fatalf("status=%q want=%q", got, tc.want)
			}
		})
	}
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "")
	if got := customerInvestigationStatus(valid, true); got != "evidence_pending" {
		t.Fatalf("missing configured signer must withhold ready status, got %q", got)
	}
}

func TestCustomerInvestigationEnvelopeUnsignedReturnsReportNotError(t *testing.T) {
	assembly := unifiedInvestigationAssembly{
		Report: map[string]any{"ok": true, "schema_version": unifiedInvestigationSchemaVersion},
		Core: holderIntelligenceCoreResult{
			Request: services.SecurityRadarRequest{Target: "Mint111", Network: "solana-mainnet"},
		},
		UnifiedVerdict: services.UnifiedRadarVerdict{Grade: "-", Signed: false, Verdict: "single_observation"},
	}
	got := customerInvestigationEnvelope(assembly, false)
	if got["ok"] != true {
		t.Fatalf("expected successful report envelope: %#v", got)
	}
	if got["status"] != "evidence_pending" {
		t.Fatalf("status=%v", got["status"])
	}
	if _, exists := got["error"]; exists {
		t.Fatalf("unsigned investigation must not become an error envelope: %#v", got)
	}
	if got["investigation_report"] == nil {
		t.Fatal("investigation report missing")
	}
	if got["charged"] != false {
		t.Fatalf("evidence-pending source-unavailable report must not be charged: %#v", got)
	}
}
