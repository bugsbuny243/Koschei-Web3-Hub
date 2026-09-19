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

func TestCustomerScanResultExposesUniversalDispatchPlan(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "0x1111111111111111111111111111111111111111",
		NetworkHint:    "polygon-mainnet",
		Kind:           CustomerScanTargetEVMAddress,
		Route:          CustomerScanRouteEVMProbe,
		Classification: "syntax_only",
	}
	result, err := BuildCustomerScanResult(target, Web3TrustVector{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.InvestigationPlan == nil {
		t.Fatal("expected universal investigation plan")
	}
	if !result.InvestigationPlan.Executable || result.InvestigationPlan.Route != UniversalDispatchEVMProbe {
		t.Fatalf("unexpected plan: %#v", result.InvestigationPlan)
	}
	if result.InvestigationPlan.VerdictAuthority != UniversalVerdictEvidenceOnly {
		t.Fatalf("unexpected verdict authority: %#v", result.InvestigationPlan)
	}
}

func TestCustomerScanResultDoesNotInventPlanBeforeNetworkResolution(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("0x1111111111111111111111111111111111111111", "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildCustomerScanResult(target, Web3TrustVector{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.InvestigationPlan != nil {
		t.Fatalf("networkless target unexpectedly received plan: %#v", result.InvestigationPlan)
	}
}


func TestCustomerScanResultFromEVMTransactionBindsObservedEvidence(t *testing.T) {
	target, err := ClassifyCustomerScanTarget(
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"ethereum-mainnet",
	)
	if err != nil {
		t.Fatal(err)
	}
	subject, err := ClassifyUniversalInvestigationSubject(target.Raw, target.NetworkHint, IntelligenceSubjectTransaction)
	if err != nil {
		t.Fatal(err)
	}
	projection := NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID:              "evm-tx-1",
			SubjectID:       subject.ID,
			ChainFamily:     IntelligenceChainFamilyEVM,
			Network:         target.NetworkHint,
			Status:          IntelligenceEvidenceObserved,
			TransactionHash: target.Raw,
		},
	}
	result, err := CustomerScanResultFromEVMTransaction(target, projection)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CustomerScanStatusObserved || result.TransactionEvidence == nil {
		t.Fatalf("unexpected transaction result: %#v", result)
	}
	if result.InvestigationPlan == nil || result.InvestigationPlan.Route != UniversalDispatchEVMTransaction {
		t.Fatalf("missing EVM transaction dispatch plan: %#v", result.InvestigationPlan)
	}
}
