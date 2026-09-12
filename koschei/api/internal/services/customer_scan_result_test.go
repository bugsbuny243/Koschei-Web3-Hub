package services

import "testing"

func TestCustomerScanResultFromNetworkProbeStaysObserved(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	subject := ClassifyIntelligenceSubject(target.Raw, target.NetworkHint)
	projection := NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID:        "ev-1",
			SubjectID: subject.ID,
			Status:    IntelligenceEvidenceObserved,
		},
	}

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
	subject := ClassifyIntelligenceSubject("0x2222222222222222222222222222222222222222", target.NetworkHint)
	projection := NetworkProbeIntelligenceProjection{
		Subject:  subject,
		Evidence: IntelligenceEvidence{ID: "ev-2", SubjectID: subject.ID, Status: IntelligenceEvidenceObserved},
	}
	if _, err := CustomerScanResultFromNetworkProbe(target, projection); err == nil {
		t.Fatal("expected mismatched probe subject to fail")
	}
}
