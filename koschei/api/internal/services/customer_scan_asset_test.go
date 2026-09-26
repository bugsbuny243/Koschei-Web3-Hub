package services

import "testing"

func TestCustomerScanResultWithEVMAssetEvidenceAttachesObservedAsset(t *testing.T) {
	target := CustomerScanTarget{
		Raw: "0x1111111111111111111111111111111111111111",
		NetworkHint: "ethereum-mainnet",
		Kind: CustomerScanTargetEVMAddress,
		Route: CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	subject := ClassifyIntelligenceSubject(target.Raw, target.NetworkHint)
	base, err := BuildCustomerScanResult(target, Web3TrustVector{Observed: true}, []string{"base-evidence"})
	if err != nil {
		t.Fatal(err)
	}
	projection := NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID: "asset-evidence", SubjectID: subject.ID, ChainFamily: IntelligenceChainFamilyEVM,
			Chain: subject.Chain, Network: subject.Network, Status: IntelligenceEvidenceObserved,
			Address: target.Raw, Attributes: map[string]any{"interface_state": "erc20_like_surface_observed"},
		},
	}
	result, err := CustomerScanResultWithEVMAssetEvidence(base, target, projection)
	if err != nil {
		t.Fatal(err)
	}
	if result.AssetEvidence == nil || result.AssetEvidence.ID != "asset-evidence" {
		t.Fatalf("asset evidence missing: %#v", result)
	}
	found := false
	for _, reason := range result.Reasons {
		if reason == "EVM_ERC20_LIKE_SURFACE_OBSERVED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("asset observation reason missing: %#v", result.Reasons)
	}
}
