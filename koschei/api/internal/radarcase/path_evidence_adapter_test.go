package radarcase

import (
	"strings"
	"testing"

	"koschei/api/internal/radarevent"
	"koschei/api/internal/radargraph"
	"koschei/api/internal/radarpath"
	"koschei/api/internal/securityevidence"
)

func pathNode(t *testing.T, networkID, ref, seed string, state securityevidence.EvidenceState) radargraph.Node {
	t.Helper()
	event, err := (radarevent.Event{
		Producer:         "test-adapter",
		Kind:             radarevent.KindAccount,
		NetworkID:        networkID,
		SubjectKind:      "address",
		SubjectID:        ref,
		ObservedAtUnixMS: 1780000000000,
		State:            state,
		NativeRefs:       []radarevent.NativeReference{{Kind: "address", Value: ref}},
		SourceDigests:    []string{strings.Repeat(seed, 64)},
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	node, err := radargraph.NodeFromEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func buildPath(t *testing.T, edgeState securityevidence.EvidenceState) radarpath.Path {
	t.Helper()
	a := pathNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "a", securityevidence.StateVerified)
	b := pathNode(t, "base-mainnet", "0x2222222222222222222222222222222222222222", "b", securityevidence.StateVerified)
	edge, err := radargraph.NewEvidenceEdge(a, b, radargraph.RelationBridgedTo, edgeState, 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := (radargraph.Graph{
		SchemaVersion: radargraph.SchemaVersionV1,
		Nodes:         []radargraph.Node{a, b},
		Edges:         []radargraph.Edge{edge},
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	path, err := radarpath.BuildPath(graph, []string{edge.ID})
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildPathEvidencePromotesEvidenceBackedPathWithoutMintingVerdict(t *testing.T) {
	path := buildPath(t, securityevidence.StateVerified)
	event, err := BuildPathEvidence(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if event.Subject.Chain != "multi-chain" || event.Subject.Type != "attack_path" {
		t.Fatalf("unexpected subject: %#v", event.Subject)
	}
	if len(event.Findings) != 1 || event.Findings[0].State != securityevidence.StateVerified {
		t.Fatalf("unexpected findings: %#v", event.Findings)
	}
	if event.SourceDigests[0] != path.PathSHA256 || event.Findings[0].EvidenceSHA256 != path.PathSHA256 {
		t.Fatal("path digest was not preserved as evidence provenance")
	}
	if event.Legacy != nil {
		t.Fatal("path adapter minted legacy score or grade")
	}
}

func TestObservedSegmentKeepsPathEvidenceObserved(t *testing.T) {
	path := buildPath(t, securityevidence.StateObserved)
	event, err := BuildPathEvidence(path)
	if err != nil {
		t.Fatal(err)
	}
	if event.Findings[0].State != securityevidence.StateObserved {
		t.Fatalf("state=%q", event.Findings[0].State)
	}
}

func TestCandidatePathCannotReachSecurityEvidence(t *testing.T) {
	a := pathNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "c", securityevidence.StateObserved)
	b := pathNode(t, "base-mainnet", "0x2222222222222222222222222222222222222222", "d", securityevidence.StateObserved)
	edge, err := radargraph.NewHypothesisEdge(a, b, radargraph.RelationBridgedTo, 7000, "amount_time_match", 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := (radargraph.Graph{Nodes: []radargraph.Node{a, b}, Edges: []radargraph.Edge{edge}}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	path, err := radarpath.BuildPath(graph, []string{edge.ID})
	if err != nil {
		t.Fatal(err)
	}
	if path.Status != radarpath.StatusCandidate {
		t.Fatalf("path status=%q", path.Status)
	}
	if _, err := BuildPathEvidence(path); err == nil || !strings.Contains(err.Error(), "candidate path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
