package services

import (
	"encoding/json"
	"testing"
)

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
			Source:    "customer_rpc_fixture",
			Network:   target.NetworkHint,
			Provenance: "synthetic_network_observation",
			Attributes: map[string]any{"code_hash": "synthetic-hash", "extra_collector_fact": 17},
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
	if result.NetworkEvidence == nil || result.NetworkEvidence.ID != "ev-1" ||
		result.NetworkEvidence.Attributes["extra_collector_fact"] != 17 ||
		result.NetworkEvidence.Provenance != "synthetic_network_observation" {
		t.Fatalf("base network probe evidence was dropped from customer result: %#v", result.NetworkEvidence)
	}
	if result.Verdict != CustomerScanVerdictReview || result.Trust.Verified || result.Trust.Finalized {
		t.Fatal("preserving native observation must not manufacture trusted final authority")
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
			Chain:           subject.Chain,
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

func TestCustomerScanResultFromEVMTransactionRejectsCrossNetworkEvidence(t *testing.T) {
	target, err := ClassifyCustomerScanTarget(
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"ethereum-mainnet",
	)
	if err != nil {
		t.Fatal(err)
	}
	subject, err := ClassifyUniversalInvestigationSubject(target.Raw, "base-mainnet", IntelligenceSubjectTransaction)
	if err != nil {
		t.Fatal(err)
	}
	projection := NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID:              "evm-tx-cross-network",
			SubjectID:       subject.ID,
			ChainFamily:     subject.ChainFamily,
			Chain:           subject.Chain,
			Network:         subject.Network,
			Status:          IntelligenceEvidenceObserved,
			TransactionHash: target.Raw,
		},
	}
	if _, err := CustomerScanResultFromEVMTransaction(target, projection); err == nil {
		t.Fatal("cross-network EVM transaction evidence was accepted")
	}
}

func TestCustomerScanResultFromEVMTransactionRejectsEvidenceSubjectMismatch(t *testing.T) {
	target, err := ClassifyCustomerScanTarget(
		"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
			ID:              "evm-tx-subject-mismatch",
			SubjectID:       "different-subject",
			ChainFamily:     subject.ChainFamily,
			Chain:           subject.Chain,
			Network:         subject.Network,
			Status:          IntelligenceEvidenceObserved,
			TransactionHash: target.Raw,
		},
	}
	if _, err := CustomerScanResultFromEVMTransaction(target, projection); err == nil {
		t.Fatal("mismatched evidence subject was accepted")
	}
}

func TestCustomerNetworkScanSerializesEntireBaseEvidenceWithoutPromotion(t *testing.T) {
	for _, network := range []string{"ethereum-mainnet", "bitcoin-mainnet"} {
		t.Run(network, func(t *testing.T) {
			var raw, kind, route string
			if network == "bitcoin-mainnet" {
				raw, kind, route = "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", CustomerScanTargetBitcoin, CustomerScanRouteBitcoinProbe
			} else {
				raw, kind, route = "0x1111111111111111111111111111111111111111", CustomerScanTargetEVMAddress, CustomerScanRouteEVMProbe
			}
			target := CustomerScanTarget{Raw: raw, NetworkHint: network, Kind: kind, Route: route}
			subject := ClassifyIntelligenceSubject(raw, network)
			projection := NetworkProbeIntelligenceProjection{
				Subject: subject,
				Evidence: IntelligenceEvidence{
					ID: "native-customer-evidence", SubjectID: subject.ID,
					Network: network, Status: IntelligenceEvidenceObserved,
					Source: "native-fixture-provider", Provenance: "synthetic-native-evidence",
					Attributes: map[string]any{"unabridged_native_observation": map[string]any{"block_height": 23, "signals": []any{"a", "b"}}},
				},
			}
			result, err := CustomerScanResultFromNetworkProbe(target, projection)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			evidence, ok := wire["network_evidence"].(map[string]any)
			if !ok || evidence["provenance"] != "synthetic-native-evidence" {
				t.Fatalf("%s full evidence missing from customer JSON: %s", network, encoded)
			}
			attrs, ok := evidence["attributes"].(map[string]any)
			if !ok || attrs["unabridged_native_observation"] == nil {
				t.Fatalf("%s deep native fact was truncated: %s", network, encoded)
			}
			if wire["verdict"] != CustomerScanVerdictReview || wire["evidence_status"] != Web3TrustEvidenceObserved {
				t.Fatalf("%s observed evidence was promoted into a final safety verdict: %s", network, encoded)
			}
		})
	}
}
