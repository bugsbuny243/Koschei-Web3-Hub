
package services

import (
	"strings"
	"testing"
	"time"
)

func evmGraphProjection(t *testing.T, network string) NetworkProbeIntelligenceProjection {
	t.Helper()
	txHash := "0x" + strings.Repeat("a", 64)
	subject, err := ClassifyUniversalInvestigationSubject(txHash, network, IntelligenceSubjectTransaction)
	if err != nil {
		t.Fatal(err)
	}
	return NetworkProbeIntelligenceProjection{
		Subject: subject,
		Evidence: IntelligenceEvidence{
			ID:              "evm-tx-evidence-1",
			SubjectID:       subject.ID,
			ChainFamily:     subject.ChainFamily,
			Chain:           subject.Chain,
			Network:         subject.Network,
			Source:          "ethereum_jsonrpc",
			Status:          IntelligenceEvidenceObserved,
			TransactionHash: txHash,
			Address:         "0x1111111111111111111111111111111111111111",
			Contract:        "0x2222222222222222222222222222222222222222",
			Attributes: map[string]any{
				"from":             "0x1111111111111111111111111111111111111111",
				"to":               "0x2222222222222222222222222222222222222222",
				"contract_address": "0x3333333333333333333333333333333333333333",
				"logs": []map[string]any{
					{
						"address": "0x4444444444444444444444444444444444444444",
						"removed": false,
						"topics":  []string{"0x" + strings.Repeat("b", 64)},
					},
					{
						"address": "0x5555555555555555555555555555555555555555",
						"removed": true,
					},
				},
			},
		},
	}
}

func TestBuildEVMTransactionGraphCreatesObservedRoleRelationships(t *testing.T) {
	projection := evmGraphProjection(t, "ethereum-mainnet")
	graph, err := BuildEVMTransactionGraph(projection, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	investigation := graph.Investigation
	if len(investigation.Subjects) != 5 {
		t.Fatalf("subjects=%d want=5: %#v", len(investigation.Subjects), investigation.Subjects)
	}
	if len(investigation.Relationships) != 4 {
		t.Fatalf("relationships=%d want=4: %#v", len(investigation.Relationships), investigation.Relationships)
	}
	for _, relation := range investigation.Relationships {
		if relation.Status != IntelligenceEvidenceObserved {
			t.Fatalf("relationship was promoted beyond observed: %#v", relation)
		}
		if len(relation.EvidenceRefs) != 1 || relation.EvidenceRefs[0] != projection.Evidence.ID {
			t.Fatalf("relationship lost evidence binding: %#v", relation)
		}
	}
	for _, subject := range investigation.Subjects {
		if subject.Raw == "0x5555555555555555555555555555555555555555" {
			t.Fatalf("removed log emitter was projected into active graph: %#v", subject)
		}
	}
}

func TestBuildEVMTransactionGraphSeparatesSameAddressAcrossNetworks(t *testing.T) {
	ethereum := evmGraphProjection(t, "ethereum-mainnet")
	base := evmGraphProjection(t, "base-mainnet")

	ethereumGraph, err := BuildEVMTransactionGraph(ethereum, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	baseGraph, err := BuildEVMTransactionGraph(base, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}

	find := func(investigation IntelligenceInvestigation, raw string) IntelligenceSubject {
		for _, subject := range investigation.Subjects {
			if strings.EqualFold(subject.Raw, raw) {
				return subject
			}
		}
		return IntelligenceSubject{}
	}
	address := "0x1111111111111111111111111111111111111111"
	ethSender := find(ethereumGraph.Investigation, address)
	baseSender := find(baseGraph.Investigation, address)
	if ethSender.ID == "" || baseSender.ID == "" {
		t.Fatal("sender subject missing")
	}
	if ethSender.ID == baseSender.ID || ethSender.CanonicalRef == baseSender.CanonicalRef {
		t.Fatalf("same address was conflated across EVM networks: eth=%#v base=%#v", ethSender, baseSender)
	}
}

func TestBuildEVMTransactionGraphRejectsMismatchedEvidenceBinding(t *testing.T) {
	projection := evmGraphProjection(t, "ethereum-mainnet")
	projection.Evidence.Network = "base-mainnet"
	if _, err := BuildEVMTransactionGraph(projection, time.Now().UTC()); err == nil {
		t.Fatal("mismatched evidence network was accepted")
	}
}
