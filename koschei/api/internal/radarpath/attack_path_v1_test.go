package radarpath

import (
	"strings"
	"testing"

	"koschei/api/internal/radargraph"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/securityevidence"
)

func graphNode(t *testing.T, networkID, ref, seed string) radargraph.Node {
	t.Helper()
	event, err := (radarevent.Event{
		Producer:         "test-adapter",
		Kind:             radarevent.KindAccount,
		NetworkID:        networkID,
		SubjectKind:      "address",
		SubjectID:        ref,
		ObservedAtUnixMS: 1780000000000,
		State:            securityevidence.StateObserved,
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

func TestFindEvidencePathAcrossBridge(t *testing.T) {
	a := graphNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "a")
	b := graphNode(t, "ethereum-mainnet", "0x2222222222222222222222222222222222222222", "b")
	c := graphNode(t, "base-mainnet", "0x3333333333333333333333333333333333333333", "c")
	d := graphNode(t, "base-mainnet", "0x4444444444444444444444444444444444444444", "d")

	funding, err := radargraph.NewEvidenceEdge(a, b, radargraph.RelationFunded, securityevidence.StateObserved, 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := radargraph.NewEvidenceEdge(b, c, radargraph.RelationBridgedTo, securityevidence.StateVerified, 1780000000200, append(b.SourceEvents, c.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	swap, err := radargraph.NewEvidenceEdge(c, d, radargraph.RelationSwappedTo, securityevidence.StateObserved, 1780000000300, append(c.SourceEvents, d.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}

	graph, err := (radargraph.Graph{
		SchemaVersion: radargraph.SchemaVersionV1,
		Nodes:         []radargraph.Node{a, b, c, d},
		Edges:         []radargraph.Edge{funding, bridge, swap},
	}).Seal()
	if err != nil {
		t.Fatal(err)
	}

	paths, err := FindPaths(graph, a.ID, d.ID, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths=%d", len(paths))
	}
	path := paths[0]
	if path.Status != StatusEvidenceBacked {
		t.Fatalf("status=%q", path.Status)
	}
	if len(path.Networks) != 2 || path.Networks[0] != "ethereum-mainnet" || path.Networks[1] != "base-mainnet" {
		t.Fatalf("networks=%v", path.Networks)
	}
	wantStages := []string{StageFunding, StageBridge, StageSwap}
	for i, want := range wantStages {
		if path.Segments[i].Stage != want {
			t.Fatalf("segment %d stage=%q want=%q", i, path.Segments[i].Stage, want)
		}
	}
	if err := path.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestHypothesisMakesPathCandidateAndCanBeExcluded(t *testing.T) {
	a := graphNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "1")
	b := graphNode(t, "base-mainnet", "0x2222222222222222222222222222222222222222", "2")
	hypothesis, err := radargraph.NewHypothesisEdge(a, b, radargraph.RelationBridgedTo, 7200, "bridge_amount_time_match", 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := (radargraph.Graph{Nodes: []radargraph.Node{a, b}, Edges: []radargraph.Edge{hypothesis}}).Seal()
	if err != nil {
		t.Fatal(err)
	}

	without, err := FindPaths(graph, a.ID, b.ID, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(without) != 0 {
		t.Fatal("hypothesis leaked into evidence-only path search")
	}

	with, err := FindPaths(graph, a.ID, b.ID, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(with) != 1 || with[0].Status != StatusCandidate {
		t.Fatalf("candidate paths=%#v", with)
	}
}

func TestCorrelationEdgesCannotBecomeCausalPathSegments(t *testing.T) {
	a := graphNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "3")
	b := graphNode(t, "base-mainnet", "0x2222222222222222222222222222222222222222", "4")
	correlation, err := radargraph.NewHypothesisEdge(a, b, radargraph.RelationCorrelatedWith, 6000, "timing_similarity", 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := (radargraph.Graph{Nodes: []radargraph.Node{a, b}, Edges: []radargraph.Edge{correlation}}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPath(graph, []string{correlation.ID}); err == nil || !strings.Contains(err.Error(), "not path-traversable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPathDigestDetectsMutation(t *testing.T) {
	a := graphNode(t, "ethereum-mainnet", "0x1111111111111111111111111111111111111111", "5")
	b := graphNode(t, "ethereum-mainnet", "0x2222222222222222222222222222222222222222", "6")
	edge, err := radargraph.NewEvidenceEdge(a, b, radargraph.RelationTransferredTo, securityevidence.StateObserved, 1780000000100, append(a.SourceEvents, b.SourceEvents...))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := (radargraph.Graph{Nodes: []radargraph.Node{a, b}, Edges: []radargraph.Edge{edge}}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	path, err := BuildPath(graph, []string{edge.ID})
	if err != nil {
		t.Fatal(err)
	}
	path.Segments[0].Relation = radargraph.RelationFunded
	if err := path.Verify(); err == nil {
		t.Fatal("mutated path verified")
	}
}
