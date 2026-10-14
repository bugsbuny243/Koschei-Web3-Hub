package services

import (
	"context"
	"encoding/json"
	r "koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/builtin"
	"strings"
	"testing"
)

func TestRuntimeCollectorsReachSharedEvidenceBridge(t *testing.T) {
	if len(radarRegistrationErrors) > 0 {
		t.Fatal(radarRegistrationErrors)
	}
	for _, capability := range RadarCapabilities() {
		t.Run(capability.Network, func(t *testing.T) {
			target := builtin.SampleTarget(capability)
			transport := builtin.FixtureTransport(capability.Family, capability.Network)
			if capability.Family == "offchain" {
				transport = r.TransportFunc(func(context.Context, r.Request) (r.Response, error) {
					return r.Response{Raw: []byte("<html></html>"), Host: "dapp.fixture.invalid"}, nil
				})
			}
			result := collectRadarInvestigation(context.Background(), target, transport)
			if !result.OK || result.GradeIssued || result.Verdict != "withhold" || len(result.Evidence) == 0 {
				t.Fatalf("not connected to evidence bridge: %+v", result)
			}
			for _, row := range result.Evidence {
				if row.SubjectID != result.Subject.ID || row.ChainFamily != capability.Family || row.Status != IntelligenceEvidenceObserved || row.Attributes["raw_response_sha256"] == "" {
					t.Fatalf("unbound/promoted evidence %+v", row)
				}
			}
		})
	}
}
func TestRuntimeEVMWrongChainCannotProduceEvidence(t *testing.T) {
	target := r.Target{Network: "ethereum-mainnet", Kind: "wallet", Raw: "0x" + strings.Repeat("1", 40)}
	tr := r.TransportFunc(func(context.Context, r.Request) (r.Response, error) {
		return r.Response{Raw: []byte(`{"result":"0x38"}`), Host: "wrong.fixture.invalid"}, nil
	})
	result := collectRadarInvestigation(context.Background(), target, tr)
	if result.OK || len(result.Evidence) > 0 || result.Status != "insufficient_evidence" {
		t.Fatal(result)
	}
}
func TestCapabilityJSONHasRuntimeTargets(t *testing.T) {
	b, e := json.Marshal(RadarCapabilities())
	if e != nil || !strings.Contains(string(b), `"target_classes"`) {
		t.Fatal(string(b), e)
	}
}
