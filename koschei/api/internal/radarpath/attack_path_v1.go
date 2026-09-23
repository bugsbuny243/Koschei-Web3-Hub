package radarpath

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"koschei/api/internal/radargraph"
	"koschei/api/internal/securityevidence"
)

const SchemaVersionV1 = "koschei.global-attack-path.v1"

const (
	StatusEvidenceBacked = "EVIDENCE_BACKED"
	StatusCandidate      = "CANDIDATE"
)

const (
	StageFunding     = "funding"
	StageDeployment  = "deployment"
	StageLiquidity   = "liquidity"
	StageInteraction = "interaction"
	StageDrain       = "drain"
	StageSwap        = "swap"
	StageBridge      = "bridge"
	StageTransfer    = "transfer"
	StageHolding     = "holding"
)

const MaxDiscoveredPaths = 100

type Segment struct {
	Index              int                            `json:"index"`
	EdgeID             string                         `json:"edge_id"`
	Relation           string                         `json:"relation"`
	Stage              string                         `json:"stage"`
	SourceID           string                         `json:"source_id"`
	TargetID           string                         `json:"target_id"`
	SourceNetworkID    string                         `json:"source_network_id"`
	TargetNetworkID    string                         `json:"target_network_id"`
	NetworkTransition  bool                           `json:"network_transition"`
	Basis              string                         `json:"basis"`
	EvidenceState      securityevidence.EvidenceState `json:"evidence_state,omitempty"`
	ConfidenceBPS      int                            `json:"confidence_bps,omitempty"`
	HypothesisReason   string                         `json:"hypothesis_reason,omitempty"`
	ObservedAtUnixMS   int64                          `json:"observed_at_unix_ms"`
	SourceEventsSHA256 []string                       `json:"source_events_sha256"`
}

type Path struct {
	SchemaVersion string    `json:"schema_version"`
	SourceID      string    `json:"source_id"`
	TargetID      string    `json:"target_id"`
	Status        string    `json:"status"`
	Networks      []string  `json:"networks"`
	Segments      []Segment `json:"segments"`
	PathSHA256    string    `json:"path_sha256"`
}

func BuildPath(graph radargraph.Graph, edgeIDs []string) (Path, error) {
	if err := graph.Verify(); err != nil {
		return Path{}, fmt.Errorf("entity graph verification failed: %w", err)
	}
	if len(edgeIDs) == 0 || len(edgeIDs) > 32 {
		return Path{}, errors.New("path must contain between 1 and 32 edges")
	}

	nodes := make(map[string]radargraph.Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	edges := make(map[string]radargraph.Edge, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges[edge.ID] = edge
	}

	segments := make([]Segment, 0, len(edgeIDs))
	status := StatusEvidenceBacked
	var previousTarget string
	for i, edgeID := range edgeIDs {
		edge, ok := edges[strings.TrimSpace(edgeID)]
		if !ok {
			return Path{}, fmt.Errorf("path references unknown edge %q", edgeID)
		}
		if !pathRelationAllowed(edge.Relation) {
			return Path{}, fmt.Errorf("relation %q is not path-traversable", edge.Relation)
		}
		if i > 0 && previousTarget != edge.SourceID {
			return Path{}, errors.New("path edges are not contiguous")
		}
		source, sourceOK := nodes[edge.SourceID]
		target, targetOK := nodes[edge.TargetID]
		if !sourceOK || !targetOK {
			return Path{}, errors.New("path edge references missing entity")
		}
		stage, ok := stageForRelation(edge.Relation)
		if !ok {
			return Path{}, fmt.Errorf("relation %q has no attack-path stage", edge.Relation)
		}
		if edge.Basis == radargraph.BasisHypothesis {
			status = StatusCandidate
		}
		segments = append(segments, Segment{
			Index:              i,
			EdgeID:             edge.ID,
			Relation:           edge.Relation,
			Stage:              stage,
			SourceID:           edge.SourceID,
			TargetID:           edge.TargetID,
			SourceNetworkID:    source.NetworkID,
			TargetNetworkID:    target.NetworkID,
			NetworkTransition:  source.NetworkID != target.NetworkID,
			Basis:              edge.Basis,
			EvidenceState:      edge.EvidenceState,
			ConfidenceBPS:      edge.ConfidenceBPS,
			HypothesisReason:   edge.HypothesisReason,
			ObservedAtUnixMS:   edge.ObservedAtUnixMS,
			SourceEventsSHA256: append([]string(nil), edge.SourceEvents...),
		})
		previousTarget = edge.TargetID
	}

	path := Path{
		SchemaVersion: SchemaVersionV1,
		SourceID:      segments[0].SourceID,
		TargetID:      segments[len(segments)-1].TargetID,
		Status:        status,
		Networks:      pathNetworks(segments),
		Segments:      segments,
	}
	return path.Seal()
}

func FindPaths(graph radargraph.Graph, sourceID, targetID string, maxHops int, includeHypotheses bool) ([]Path, error) {
	if err := graph.Verify(); err != nil {
		return nil, fmt.Errorf("entity graph verification failed: %w", err)
	}
	sourceID = strings.TrimSpace(sourceID)
	targetID = strings.TrimSpace(targetID)
	if sourceID == "" || targetID == "" || sourceID == targetID {
		return nil, errors.New("source and target entities must be distinct")
	}
	if maxHops < 1 || maxHops > 12 {
		return nil, errors.New("max_hops must be between 1 and 12")
	}

	nodes := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = struct{}{}
	}
	if _, ok := nodes[sourceID]; !ok {
		return nil, errors.New("source entity is not in graph")
	}
	if _, ok := nodes[targetID]; !ok {
		return nil, errors.New("target entity is not in graph")
	}

	adjacency := make(map[string][]radargraph.Edge)
	for _, edge := range graph.Edges {
		if !pathRelationAllowed(edge.Relation) {
			continue
		}
		if !includeHypotheses && edge.Basis != radargraph.BasisEvidence {
			continue
		}
		adjacency[edge.SourceID] = append(adjacency[edge.SourceID], edge)
	}
	for source := range adjacency {
		sort.Slice(adjacency[source], func(i, j int) bool {
			return adjacency[source][i].ID < adjacency[source][j].ID
		})
	}

	type walk struct {
		node    string
		edges   []string
		visited map[string]struct{}
	}
	queue := []walk{{
		node:    sourceID,
		visited: map[string]struct{}{sourceID: {}},
	}}
	paths := make([]Path, 0)
	for len(queue) > 0 && len(paths) < MaxDiscoveredPaths {
		current := queue[0]
		queue = queue[1:]
		if len(current.edges) >= maxHops {
			continue
		}
		for _, edge := range adjacency[current.node] {
			if _, seen := current.visited[edge.TargetID]; seen {
				continue
			}
			nextEdges := append(append([]string(nil), current.edges...), edge.ID)
			if edge.TargetID == targetID {
				path, err := BuildPath(graph, nextEdges)
				if err != nil {
					return nil, err
				}
				paths = append(paths, path)
				if len(paths) >= MaxDiscoveredPaths {
					break
				}
				continue
			}
			nextVisited := make(map[string]struct{}, len(current.visited)+1)
			for id := range current.visited {
				nextVisited[id] = struct{}{}
			}
			nextVisited[edge.TargetID] = struct{}{}
			queue = append(queue, walk{
				node:    edge.TargetID,
				edges:   nextEdges,
				visited: nextVisited,
			})
		}
	}

	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i].Segments) != len(paths[j].Segments) {
			return len(paths[i].Segments) < len(paths[j].Segments)
		}
		return paths[i].PathSHA256 < paths[j].PathSHA256
	})
	return paths, nil
}

func (p Path) Seal() (Path, error) {
	canonical, err := p.canonical()
	if err != nil {
		return Path{}, err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return Path{}, err
	}
	sum := sha256.Sum256(payload)
	canonical.PathSHA256 = hex.EncodeToString(sum[:])
	return canonical, nil
}

func (p Path) Verify() error {
	expected := strings.ToLower(strings.TrimSpace(p.PathSHA256))
	if !validSHA256(expected) {
		return errors.New("path_sha256 is required and must be sha256")
	}
	canonical, err := p.canonical()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("global attack path digest mismatch")
	}
	return nil
}

func (p Path) canonical() (Path, error) {
	out := p
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		out.SchemaVersion = SchemaVersionV1
	}
	if out.SchemaVersion != SchemaVersionV1 {
		return Path{}, fmt.Errorf("unsupported path schema %q", out.SchemaVersion)
	}
	out.PathSHA256 = ""
	out.SourceID = strings.TrimSpace(out.SourceID)
	out.TargetID = strings.TrimSpace(out.TargetID)
	out.Status = strings.ToUpper(strings.TrimSpace(out.Status))
	if out.SourceID == "" || out.TargetID == "" || out.SourceID == out.TargetID {
		return Path{}, errors.New("path requires distinct source and target entities")
	}
	if len(out.Segments) == 0 || len(out.Segments) > 32 {
		return Path{}, errors.New("path must contain between 1 and 32 segments")
	}

	candidate := false
	for i := range out.Segments {
		segment := &out.Segments[i]
		if segment.Index != i {
			return Path{}, errors.New("path segment indexes are not canonical")
		}
		if i == 0 && segment.SourceID != out.SourceID {
			return Path{}, errors.New("path source does not match first segment")
		}
		if i > 0 && out.Segments[i-1].TargetID != segment.SourceID {
			return Path{}, errors.New("path segments are not contiguous")
		}
		if i == len(out.Segments)-1 && segment.TargetID != out.TargetID {
			return Path{}, errors.New("path target does not match last segment")
		}
		if !pathRelationAllowed(segment.Relation) {
			return Path{}, fmt.Errorf("relation %q is not path-traversable", segment.Relation)
		}
		stage, ok := stageForRelation(segment.Relation)
		if !ok || segment.Stage != stage {
			return Path{}, errors.New("path segment stage does not match relation")
		}
		if segment.NetworkTransition != (segment.SourceNetworkID != segment.TargetNetworkID) {
			return Path{}, errors.New("path network transition marker is inconsistent")
		}
		switch segment.Basis {
		case radargraph.BasisEvidence:
			if segment.EvidenceState != securityevidence.StateObserved && segment.EvidenceState != securityevidence.StateVerified {
				return Path{}, errors.New("evidence path segment lacks evidence state")
			}
			if segment.ConfidenceBPS != 0 || segment.HypothesisReason != "" {
				return Path{}, errors.New("evidence path segment carries hypothesis metadata")
			}
		case radargraph.BasisHypothesis:
			candidate = true
			if segment.EvidenceState != "" || segment.ConfidenceBPS <= 0 || segment.ConfidenceBPS >= 10000 || strings.TrimSpace(segment.HypothesisReason) == "" {
				return Path{}, errors.New("invalid hypothesis path segment")
			}
		default:
			return Path{}, errors.New("path segment basis is invalid")
		}
		if segment.ObservedAtUnixMS <= 0 || len(segment.SourceEventsSHA256) == 0 {
			return Path{}, errors.New("path segment lacks observation provenance")
		}
		for _, digest := range segment.SourceEventsSHA256 {
			if !validSHA256(strings.ToLower(strings.TrimSpace(digest))) {
				return Path{}, errors.New("path segment contains invalid source event digest")
			}
		}
	}

	expectedStatus := StatusEvidenceBacked
	if candidate {
		expectedStatus = StatusCandidate
	}
	if out.Status != expectedStatus {
		return Path{}, errors.New("path status does not match segment evidence basis")
	}
	expectedNetworks := pathNetworks(out.Segments)
	if len(out.Networks) != len(expectedNetworks) {
		return Path{}, errors.New("path network sequence is not canonical")
	}
	for i := range expectedNetworks {
		if out.Networks[i] != expectedNetworks[i] {
			return Path{}, errors.New("path network sequence is not canonical")
		}
	}
	return out, nil
}

func pathNetworks(segments []Segment) []string {
	if len(segments) == 0 {
		return nil
	}
	out := []string{segments[0].SourceNetworkID}
	for _, segment := range segments {
		if len(out) == 0 || out[len(out)-1] != segment.TargetNetworkID {
			out = append(out, segment.TargetNetworkID)
		}
	}
	return out
}

func stageForRelation(relation string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case radargraph.RelationFunded:
		return StageFunding, true
	case radargraph.RelationCreated, radargraph.RelationDeployed:
		return StageDeployment, true
	case radargraph.RelationLiquidityTo:
		return StageLiquidity, true
	case radargraph.RelationInteractedWith, radargraph.RelationCalled:
		return StageInteraction, true
	case radargraph.RelationDrainedTo:
		return StageDrain, true
	case radargraph.RelationSwappedTo:
		return StageSwap, true
	case radargraph.RelationBridgedTo:
		return StageBridge, true
	case radargraph.RelationTransferredTo:
		return StageTransfer, true
	case radargraph.RelationHolds:
		return StageHolding, true
	default:
		return "", false
	}
}

func pathRelationAllowed(relation string) bool {
	_, ok := stageForRelation(relation)
	return ok
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
