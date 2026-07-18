package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func TestCustomerInvestigationStatusRequiresSignedLiveVerdict(t *testing.T) {
	tests := []struct {
		name  string
		final services.UnifiedRadarVerdict
		live  bool
		want  string
	}{
		{name: "unsigned live evidence remains pending", final: services.UnifiedRadarVerdict{Grade: "-", Signed: false}, live: true, want: "evidence_pending"},
		{name: "signed result without live evidence remains pending", final: services.UnifiedRadarVerdict{Grade: "B", Signed: true}, live: false, want: "evidence_pending"},
		{name: "signed live verdict is ready", final: services.UnifiedRadarVerdict{Grade: "B", Signed: true}, live: true, want: "ready"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := customerInvestigationStatus(test.final, test.live); got != test.want {
				t.Fatalf("status=%q want=%q", got, test.want)
			}
		})
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
