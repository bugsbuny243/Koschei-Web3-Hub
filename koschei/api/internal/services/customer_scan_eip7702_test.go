package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestCustomerScanSurfacesEIP7702WithoutManufacturingAuthorization(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = IntelligenceEvidenceObserved
	resolution.LiveAvailability = "checked"

	projection, err := AdaptEVMProbeEvidence(networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "contract_code_observed",
		DelegationState:   networktarget.EVMDelegationStateObserved,
		DelegationTarget:  "0x2222222222222222222222222222222222222222",
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	target, err := ClassifyCustomerScanTarget("0x1111111111111111111111111111111111111111", "ethereum-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	result, err := CustomerScanResultFromNetworkProbe(target, projection)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Trust.Observed || result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized {
		t.Fatalf("delegation evidence was over-promoted: %+v", result.Trust)
	}
	found := false
	for _, reason := range result.Reasons {
		if reason == "EIP7702_DELEGATION_OBSERVED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("delegation reason missing: %v", result.Reasons)
	}
	if got := projection.Evidence.Attributes["delegation_target"]; got != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("delegation target attribute=%v", got)
	}
}
