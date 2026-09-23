package radargraph

import (
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/securityevidence"
)

func sealedEvent(t *testing.T, networkID, subjectKind, subjectID string, state securityevidence.EvidenceState, seed string) radarevent.Event {
	t.Helper()
	event, err := (radarevent.Event{
		Producer:         "test-adapter",
		Kind:             radarevent.KindAccount,
		NetworkID:        networkID,
		SubjectKind:      subjectKind,
		SubjectID:        subjectID,
		ObservedAtUnixMS: 1780000000000,
		State:            state,
		NativeRefs:       []radarevent.NativeReference{{Kind: subjectKind, Value: subjectID}},
		SourceDigests:    []string{strings.Repeat(seed, 64)},
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestNodeIdentityIsNetworkScopedAndDeterministic(t *testing.T) {
	ethereum := sealedEvent(t, "ethereum-mainnet", "address", "0x1111111111111111111111111111111111111111", securityevidence.StateObserved, "a")
	base := sealedEvent(t, "base-mainnet", "address", "0x1111111111111111111111111111111111111111", securityevidence.StateObserved, "b")

	ethNode, err := NodeFromEvent(ethereum)
	if err != nil {
		t.Fatal(err)
	}
	baseNode, err := NodeFromEvent(base)
	if err != nil {
		t.Fatal(err)
	}
	if ethNode.ID == baseNode.ID {
		t.Fatal("same address syntax on two networks collapsed into one entity")
	}

	again, err := NodeFromEvent(ethereum)
	if err != nil {
		t.Fatal(err)
	}
	if ethNode.ID != again.ID {
		t.Fatal("entity id is not deterministic")
	}
}

func TestEvidenceAndHypothesisRemainSeparate(t *testing.T) {
	sourceEvent := sealedEvent(t, "ethereum-mainnet", "address", "0x1111111111111111111111111111111111111111", securityevidence.StateVerified, "c")
	targetEvent := sealedEvent(t, "ethereum-mainnet", "address", "0x2222222222222222222222222222222222222222", securityevidence.StateObserved, "d")
	source, _ := NodeFromEvent(sourceEvent)
	target, _ := NodeFromEvent(targetEvent)

	evidence, err := NewEvidenceEdge(source, target, RelationTransferredTo, securityevidence.StateObserved, 1780000000100, []string{sourceEvent.EventSHA256, targetEvent.EventSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Basis != BasisEvidence || evidence.ConfidenceBPS != 0 {
		t.Fatalf("unexpected evidence edge: %#v", evidence)
	}

	hypothesis, err := NewHypothesisEdge(source, target, RelationSuspectedSameActor, 6500, "shared_funding_pattern", 1780000000200, []string{sourceEvent.EventSHA256, targetEvent.EventSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if hypothesis.Basis != BasisHypothesis || hypothesis.EvidenceState != "" {
		t.Fatalf("hypothesis claimed evidence state: %#v", hypothesis)
	}

	if _, err := NewEvidenceEdge(source, target, RelationSuspectedSameActor, securityevidence.StateVerified, 1780000000300, []string{sourceEvent.EventSHA256}); err == nil {
		t.Fatal("suspected same actor was promoted to verified evidence")
	}
}

func TestCrossChainEdgesAreRestrictedToExplicitRelations(t *testing.T) {
	ethEvent := sealedEvent(t, "ethereum-mainnet", "address", "0x1111111111111111111111111111111111111111", securityevidence.StateObserved, "e")
	baseEvent := sealedEvent(t, "base-mainnet", "address", "0x2222222222222222222222222222222222222222", securityevidence.StateObserved, "f")
	ethNode, _ := NodeFromEvent(ethEvent)
	baseNode, _ := NodeFromEvent(baseEvent)

	invalid, err := NewEvidenceEdge(ethNode, baseNode, RelationTransferredTo, securityevidence.StateObserved, 1780000000400, []string{ethEvent.EventSHA256, baseEvent.EventSHA256})
	if err != nil {
		t.Fatal(err)
	}
	graph := Graph{Nodes: []Node{ethNode, baseNode}, Edges: []Edge{invalid}}
	if _, err := graph.Seal(); err == nil || !strings.Contains(err.Error(), "cannot cross networks") {
		t.Fatalf("unexpected cross-chain validation error: %v", err)
	}

	bridge, err := NewEvidenceEdge(ethNode, baseNode, RelationBridgedTo, securityevidence.StateObserved, 1780000000500, []string{ethEvent.EventSHA256, baseEvent.EventSHA256})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := (Graph{Nodes: []Node{ethNode, baseNode}, Edges: []Edge{bridge}}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if err := sealed.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestGraphMergesDuplicateNodesWithoutLosingVerifiedState(t *testing.T) {
	observedEvent := sealedEvent(t, "bitcoin-mainnet", "address", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", securityevidence.StateObserved, "1")
	verifiedEvent := sealedEvent(t, "bitcoin-mainnet", "address", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", securityevidence.StateVerified, "2")
	observed, _ := NodeFromEvent(observedEvent)
	verified, _ := NodeFromEvent(verifiedEvent)

	sealed, err := (Graph{Nodes: []Node{observed, verified}}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed.Nodes) != 1 {
		t.Fatalf("node count=%d", len(sealed.Nodes))
	}
	if sealed.Nodes[0].EvidenceState != securityevidence.StateVerified {
		t.Fatalf("state=%q", sealed.Nodes[0].EvidenceState)
	}
	if len(sealed.Nodes[0].SourceEvents) != 2 {
		t.Fatalf("source events=%v", sealed.Nodes[0].SourceEvents)
	}
}
