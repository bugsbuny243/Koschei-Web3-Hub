package handlers

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

func TestAttachArvisNetworkProbeIntelligenceCarriesObservedEVMEvidenceWithoutDecision(t *testing.T) {
	observedAt := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	projection, err := services.AdaptEVMProbeEvidence(networktarget.EVMProbeResult{
		Resolution:        resolution,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
		ExpectedChainID:   "0x1",
		ChainID:           "0x1",
		ContractCodeState: "no_contract_code_observed",
	}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	investigation := services.BuildIntelligenceInvestigation(nil, observedAt)
	if !attachArvisNetworkProbeIntelligence(&investigation, projection) {
		t.Fatal("expected EVM projection to attach")
	}
	if len(investigation.Subjects) != 1 || len(investigation.Evidence) != 1 {
		t.Fatalf("unexpected investigation projection: %#v", investigation)
	}
	if investigation.Decision.Status != services.IntelligenceEvidenceUnverified || investigation.Decision.Action != "investigate" {
		t.Fatalf("observed network probe must not manufacture a decision: %#v", investigation.Decision)
	}
}

func TestAttachArvisNetworkProbeIntelligenceRejectsUnverifiedOrMismatchedEvidence(t *testing.T) {
	observedAt := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	subject := services.ClassifyIntelligenceSubject("0x1111111111111111111111111111111111111111", "ethereum-mainnet")
	for _, evidence := range []services.IntelligenceEvidence{
		{ID: "e1", SubjectID: subject.ID, ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network, Status: services.IntelligenceEvidenceVerified, Provenance: "networktarget_read_only_probe"},
		{ID: "e2", SubjectID: "other", ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network, Status: services.IntelligenceEvidenceObserved, Provenance: "networktarget_read_only_probe"},
		{ID: "e3", SubjectID: subject.ID, ChainFamily: subject.ChainFamily, Chain: subject.Chain, Network: subject.Network, Status: services.IntelligenceEvidenceObserved, Provenance: "other"},
	} {
		investigation := services.BuildIntelligenceInvestigation(nil, observedAt)
		if attachArvisNetworkProbeIntelligence(&investigation, services.NetworkProbeIntelligenceProjection{Subject: subject, Evidence: evidence}) {
			t.Fatalf("unsafe projection attached: %#v", evidence)
		}
		if len(investigation.Evidence) != 0 {
			t.Fatalf("rejected projection mutated evidence: %#v", investigation.Evidence)
		}
	}
}
