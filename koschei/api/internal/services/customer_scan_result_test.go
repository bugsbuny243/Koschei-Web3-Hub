package services

import (
	"testing"
	"time"
)

func validCustomerProbeProjection(target CustomerScanTarget) NetworkProbeIntelligenceProjection {
	subject := ClassifyIntelligenceSubject(target.Raw, target.NetworkHint)
	source := "evm_rpc_probe"
	method := "eth_getCode"
	if target.Route == CustomerScanRouteBitcoinProbe {
		source = "bitcoin_esplora_probe"
		method = "address_activity"
	}
	return NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID:         "ev-1",
			SubjectID:  subject.ID,
			Network:    subject.Network,
			Status:     IntelligenceEvidenceObserved,
			Address:    target.Raw,
			Source:     source,
			Method:     method,
			Provenance: networkProbeEvidenceProvenance,
			ObservedAt: time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC),
		},
	}
}

func TestCustomerScanResultFromNetworkProbeStaysObserved(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	projection := validCustomerProbeProjection(target)

	result, err := CustomerScanResultFromNetworkProbe(target, projection)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CustomerScanStatusObserved || result.Verdict != CustomerScanVerdictReview {
		t.Fatalf("unexpected result status=%q verdict=%q", result.Status, result.Verdict)
	}
	if !result.Trust.Observed || result.Trust.Verified || result.Trust.Finalized || result.Trust.Authorized {
		t.Fatalf("network observation was over-promoted: %+v", result.Trust)
	}
	if result.EvidenceStatus != Web3TrustEvidenceObserved {
		t.Fatalf("evidence_status=%q", result.EvidenceStatus)
	}
	if len(result.EvidenceRefs) != 1 || result.EvidenceRefs[0] != projection.Evidence.ID {
		t.Fatalf("unexpected evidence refs: %+v", result.EvidenceRefs)
	}
}

func TestCustomerScanResultRequiresNetworkContext(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("0x1111111111111111111111111111111111111111", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildCustomerScanResult(target, Web3TrustVector{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CustomerScanStatusNeedsContext || result.Verdict != CustomerScanVerdictUnknown {
		t.Fatalf("status=%q verdict=%q", result.Status, result.Verdict)
	}
}

func TestCustomerScanResultRejectsImpossibleTrustPromotion(t *testing.T) {
	target := CustomerScanTarget{Raw: "x", Route: CustomerScanRouteUnresolved}
	_, err := BuildCustomerScanResult(target, Web3TrustVector{Verified: true}, nil)
	if err == nil {
		t.Fatal("expected verified-without-observed trust vector to fail")
	}
}

func TestCustomerScanResultRejectsMismatchedProbeSubject(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	projection := validCustomerProbeProjection(target)
	projection.Subject = ClassifyIntelligenceSubject("0x2222222222222222222222222222222222222222", target.NetworkHint)
	if _, err := CustomerScanResultFromNetworkProbe(target, projection); err == nil {
		t.Fatal("expected mismatched probe subject to fail")
	}
}

func TestCustomerScanResultRejectsMismatchedProbeNetwork(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	projection := validCustomerProbeProjection(target)
	projection.Subject = ClassifyIntelligenceSubject(target.Raw, "base-mainnet")
	if _, err := CustomerScanResultFromNetworkProbe(target, projection); err == nil {
		t.Fatal("expected cross-network probe evidence to fail")
	}
}

func TestCustomerScanResultRejectsEvidenceNetworkMismatch(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	projection := validCustomerProbeProjection(target)
	projection.Evidence.Network = "base-mainnet"
	if _, err := CustomerScanResultFromNetworkProbe(target, projection); err == nil {
		t.Fatal("expected evidence network mismatch to fail")
	}
}

func TestCustomerScanResultRejectsForeignOrIncompleteProbeProvenance(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	cases := []struct {
		name   string
		mutate func(*NetworkProbeIntelligenceProjection)
	}{
		{"missing_id", func(p *NetworkProbeIntelligenceProjection) { p.Evidence.ID = "" }},
		{"wrong_address", func(p *NetworkProbeIntelligenceProjection) {
			p.Evidence.Address = "0x2222222222222222222222222222222222222222"
		}},
		{"missing_time", func(p *NetworkProbeIntelligenceProjection) { p.Evidence.ObservedAt = time.Time{} }},
		{"wrong_provenance", func(p *NetworkProbeIntelligenceProjection) { p.Evidence.Provenance = "foreign_adapter" }},
		{"wrong_source", func(p *NetworkProbeIntelligenceProjection) { p.Evidence.Source = "bitcoin_esplora_probe" }},
		{"wrong_method", func(p *NetworkProbeIntelligenceProjection) { p.Evidence.Method = "address_activity" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projection := validCustomerProbeProjection(target)
			tc.mutate(&projection)
			if _, err := CustomerScanResultFromNetworkProbe(target, projection); err == nil {
				t.Fatalf("expected %s provenance mismatch to fail", tc.name)
			}
		})
	}
}

func TestCustomerScanResultAcceptsBoundBitcoinProbeProvenance(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		NetworkHint:    "bitcoin-mainnet",
		Kind:           CustomerScanTargetBitcoin,
		Route:          CustomerScanRouteBitcoinProbe,
		Classification: "syntax_only",
	}
	projection := validCustomerProbeProjection(target)
	if _, err := CustomerScanResultFromNetworkProbe(target, projection); err != nil {
		t.Fatalf("valid bitcoin probe provenance rejected: %v", err)
	}
}
