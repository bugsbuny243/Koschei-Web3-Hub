package services

import "testing"

func TestGlobalRadarRelationRequiresExplicitBindingEvidence(t *testing.T) {
	source := testGlobalRadarSubject()
	target := source
	target.ID = "kis_target"
	target.Raw = "0x2222222222222222222222222222222222222222"
	target.CanonicalRef = "evm:ethereum:ethereum-mainnet:0x2222222222222222222222222222222222222222"

	evidence := testGlobalRadarEvidence(source)
	edge, err := BuildGlobalRadarRelationEdge(source, target, "funded_by", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if edge.Status != IntelligenceEvidenceUnverified || len(edge.EvidenceRefs) != 0 {
		t.Fatalf("unbound evidence verified relation: %#v", edge)
	}

	evidence.Attributes = map[string]any{
		"source_subject_id": source.ID,
		"target_subject_id": target.ID,
	}
	edge, err = BuildGlobalRadarRelationEdge(source, target, "funded_by", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if edge.Status != IntelligenceEvidenceVerified || len(edge.EvidenceRefs) != 1 || edge.EvidenceRefs[0] != evidence.ID {
		t.Fatalf("explicit binding evidence did not verify relation: %#v", edge)
	}
}

func TestGlobalRadarCrossNetworkRelationRequiresBothNetworksInLinkEvidence(t *testing.T) {
	source := testGlobalRadarSubject()
	target := source
	target.ID = "kis_base_target"
	target.Network = "base-mainnet"
	target.Chain = "base"
	target.CanonicalRef = "evm:base:base-mainnet:0x1111111111111111111111111111111111111111"

	evidence := testGlobalRadarEvidence(source)
	evidence.Attributes = map[string]any{
		"source_subject_id": source.ID,
		"target_subject_id": target.ID,
	}

	edge, err := BuildGlobalRadarRelationEdge(source, target, "bridge_flow", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !edge.CrossNetwork || edge.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("cross-network relation was promoted without network anchors: %#v", edge)
	}

	evidence.Attributes["source_network"] = source.Network
	evidence.Attributes["target_network"] = target.Network
	edge, err = BuildGlobalRadarRelationEdge(source, target, "bridge_flow", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !edge.CrossNetwork || edge.Status != IntelligenceEvidenceVerified {
		t.Fatalf("explicit cross-network link evidence was not accepted: %#v", edge)
	}
}

func TestGlobalRadarRelationDoesNotTreatIndependentEndpointEvidenceAsLinkProof(t *testing.T) {
	source := testGlobalRadarSubject()
	target := source
	target.ID = "kis_target"
	target.Raw = "0x3333333333333333333333333333333333333333"
	target.CanonicalRef = "evm:ethereum:ethereum-mainnet:0x3333333333333333333333333333333333333333"

	evidence := testGlobalRadarEvidence(source)
	evidence.Attributes = map[string]any{
		"source_subject_id": source.ID,
	}
	edge, err := BuildGlobalRadarRelationEdge(source, target, "same_actor", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if edge.Status != IntelligenceEvidenceUnverified {
		t.Fatalf("single-endpoint evidence fabricated a relationship: %#v", edge)
	}
}

func TestGlobalRadarRelationRejectsInvalidEndpoints(t *testing.T) {
	source := testGlobalRadarSubject()
	target := source
	if _, err := BuildGlobalRadarRelationEdge(source, target, "same_actor", IntelligenceEvidence{}); err == nil {
		t.Fatal("self relation unexpectedly accepted")
	}

	target.ID = "kis_target"
	target.CanonicalRef = ""
	if _, err := BuildGlobalRadarRelationEdge(source, target, "same_actor", IntelligenceEvidence{}); err == nil {
		t.Fatal("non-canonical target unexpectedly accepted")
	}
}
