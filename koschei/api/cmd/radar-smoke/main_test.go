package main

import (
	"koschei/api/internal/radaradapter"
	"koschei/api/internal/services"
	"strings"
	"testing"
	"time"
)

func TestSmokeRejectsMissingEvidenceAndUnsafePromotion(t *testing.T) {
	c := smokeCase{radaradapter.Target{Network: "ethereum-mainnet", Kind: "wallet", Raw: "fixture"}, []string{"evm-7702-delegation"}}
	result := services.RadarInvestigationResult{OK: true, Verdict: "withhold", Status: "partial_investigation", Subject: services.IntelligenceSubject{ID: "subject", Network: c.Target.Network}}
	if evaluate(c, result).Passed {
		t.Fatal("empty result passed")
	}
	result.Evidence = []services.IntelligenceEvidence{{ID: "row", SubjectID: "subject", Network: c.Target.Network, Source: "fixture.invalid", ObservedAt: time.Now(), Attributes: map[string]any{"relation": c.Required[0], "raw_response_sha256": strings.Repeat("a", 64), "native_chain_identity": "evm:1"}}}
	if !evaluate(c, result).Passed {
		t.Fatal("bounded valid evidence rejected")
	}
	result.GradeIssued = true
	if evaluate(c, result).Passed {
		t.Fatal("unexpected grade passed")
	}
	result.GradeIssued = false
	result.Evidence[0].SubjectID = "other"
	if evaluate(c, result).Passed {
		t.Fatal("wrong subject passed")
	}
}
func TestSmokeManifestRequiresExplicitAssertions(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"target":{"network":"tron-mainnet","kind":"wallet","target":"fixture"}}]`, `[{"unknown":true}]`} {
		if _, err := manifest(strings.NewReader(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
