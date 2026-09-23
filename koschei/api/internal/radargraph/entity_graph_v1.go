package radargraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/securityevidence"
)

const SchemaVersionV1 = "koschei.global-entity-graph.v1"

const (
	BasisEvidence   = "EVIDENCE"
	BasisHypothesis = "HYPOTHESIS"
)

const (
	RelationFunded             = "funded"
	RelationTransferredTo      = "transferred_to"
	RelationDrainedTo          = "drained_to"
	RelationSwappedTo          = "swapped_to"
	RelationCalled             = "called"
	RelationCreated            = "created"
	RelationDeployed           = "deployed"
	RelationBridgedTo          = "bridged_to"
	RelationLiquidityTo        = "liquidity_to"
	RelationHolds              = "holds"
	RelationInteractedWith     = "interacted_with"
	RelationCorrelatedWith     = "correlated_with"
	RelationSuspectedSameActor = "suspected_same_actor"
)

type Node struct {
	ID            string                         `json:"id"`
	NetworkID     string                         `json:"network_id"`
	Kind          string                         `json:"kind"`
	Ref           string                         `json:"ref"`
	EvidenceState securityevidence.EvidenceState `json:"evidence_state"`
	SourceEvents  []string                       `json:"source_events_sha256"`
}

type Edge struct {
	ID               string                         `json:"id"`
	SourceID         string                         `json:"source_id"`
	TargetID         string                         `json:"target_id"`
	Relation         string                         `json:"relation"`
	Basis            string                         `json:"basis"`
	EvidenceState    securityevidence.EvidenceState `json:"evidence_state,omitempty"`
	ConfidenceBPS    int                            `json:"confidence_bps,omitempty"`
	HypothesisReason string                         `json:"hypothesis_reason,omitempty"`
	ObservedAtUnixMS int64                          `json:"observed_at_unix_ms"`
	SourceEvents     []string                       `json:"source_events_sha256"`
}

type Graph struct {
	SchemaVersion string `json:"schema_version"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
	GraphSHA256   string `json:"graph_sha256"`
}

func NodeFromEvent(event radarevent.Event) (Node, error) {
	if err := event.Verify(); err != nil {
		return Node{}, fmt.Errorf("radar event verification failed: %w", err)
	}
	if event.State != securityevidence.StateObserved && event.State != securityevidence.StateVerified {
		return Node{}, errors.New("entity node requires observed or verified radar event")
	}
	networkID := strings.ToLower(strings.TrimSpace(event.NetworkID))
	kind := strings.ToLower(strings.TrimSpace(event.SubjectKind))
	ref := strings.TrimSpace(event.SubjectID)
	id, err := StableNodeID(networkID, kind, ref)
	if err != nil {
		return Node{}, err
	}
	return Node{
		ID:            id,
		NetworkID:     networkID,
		Kind:          kind,
		Ref:           ref,
		EvidenceState: event.State,
		SourceEvents:  []string{event.EventSHA256},
	}, nil
}

func StableNodeID(networkID, kind, ref string) (string, error) {
	networkID = strings.ToLower(strings.TrimSpace(networkID))
	kind = strings.ToLower(strings.TrimSpace(kind))
	ref = strings.TrimSpace(ref)
	if _, ok := networktarget.LookupNetwork(networkID); !ok {
		return "", fmt.Errorf("network %q is not registered", networkID)
	}
	if !validNodeKind(kind) || ref == "" {
		return "", errors.New("complete entity identity is required")
	}
	sum := sha256.Sum256([]byte(networkID + "\x00" + kind + "\x00" + ref))
	return "entity:" + hex.EncodeToString(sum[:]), nil
}

func NewEvidenceEdge(source, target Node, relation string, state securityevidence.EvidenceState, observedAtUnixMS int64, sourceEvents []string) (Edge, error) {
	if state != securityevidence.StateObserved && state != securityevidence.StateVerified {
		return Edge{}, errors.New("evidence edge requires observed or verified state")
	}
	edge := Edge{
		SourceID:         source.ID,
		TargetID:         target.ID,
		Relation:         relation,
		Basis:            BasisEvidence,
		EvidenceState:    state,
		ObservedAtUnixMS: observedAtUnixMS,
		SourceEvents:     sourceEvents,
	}
	return canonicalEdge(edge)
}

func NewHypothesisEdge(source, target Node, relation string, confidenceBPS int, reason string, observedAtUnixMS int64, sourceEvents []string) (Edge, error) {
	edge := Edge{
		SourceID:         source.ID,
		TargetID:         target.ID,
		Relation:         relation,
		Basis:            BasisHypothesis,
		ConfidenceBPS:    confidenceBPS,
		HypothesisReason: reason,
		ObservedAtUnixMS: observedAtUnixMS,
		SourceEvents:     sourceEvents,
	}
	return canonicalEdge(edge)
}

func (g Graph) Canonical() (Graph, error) {
	out := g
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		out.SchemaVersion = SchemaVersionV1
	}
	if out.SchemaVersion != SchemaVersionV1 {
		return Graph{}, fmt.Errorf("unsupported graph schema %q", out.SchemaVersion)
	}
	out.GraphSHA256 = ""

	nodesByID := make(map[string]Node, len(out.Nodes))
	nodes := make([]Node, 0, len(out.Nodes))
	for _, node := range out.Nodes {
		canonical, err := canonicalNode(node)
		if err != nil {
			return Graph{}, err
		}
		if existing, ok := nodesByID[canonical.ID]; ok {
			if existing.NetworkID != canonical.NetworkID || existing.Kind != canonical.Kind || existing.Ref != canonical.Ref {
				return Graph{}, fmt.Errorf("entity id collision %q", canonical.ID)
			}
			existing.SourceEvents = mergeDigests(existing.SourceEvents, canonical.SourceEvents)
			if evidenceRank(canonical.EvidenceState) > evidenceRank(existing.EvidenceState) {
				existing.EvidenceState = canonical.EvidenceState
			}
			nodesByID[canonical.ID] = existing
			continue
		}
		nodesByID[canonical.ID] = canonical
	}
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	out.Nodes = nodes

	seenEdges := make(map[string]Edge, len(out.Edges))
	for _, edge := range out.Edges {
		canonical, err := canonicalEdge(edge)
		if err != nil {
			return Graph{}, err
		}
		source, sourceOK := nodesByID[canonical.SourceID]
		target, targetOK := nodesByID[canonical.TargetID]
		if !sourceOK || !targetOK {
			return Graph{}, errors.New("edge references unknown entity")
		}
		if source.NetworkID != target.NetworkID && !crossChainRelationAllowed(canonical.Relation) {
			return Graph{}, fmt.Errorf("relation %q cannot cross networks", canonical.Relation)
		}
		if canonical.Relation == RelationSuspectedSameActor && canonical.Basis != BasisHypothesis {
			return Graph{}, errors.New("suspected_same_actor must remain a hypothesis")
		}
		if _, exists := seenEdges[canonical.ID]; !exists {
			seenEdges[canonical.ID] = canonical
		}
	}
	edges := make([]Edge, 0, len(seenEdges))
	for _, edge := range seenEdges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	out.Edges = edges
	return out, nil
}

func (g Graph) Seal() (Graph, error) {
	canonical, err := g.Canonical()
	if err != nil {
		return Graph{}, err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return Graph{}, err
	}
	sum := sha256.Sum256(payload)
	canonical.GraphSHA256 = hex.EncodeToString(sum[:])
	return canonical, nil
}

func (g Graph) Verify() error {
	expected := strings.ToLower(strings.TrimSpace(g.GraphSHA256))
	if !validSHA256(expected) {
		return errors.New("graph_sha256 is required and must be sha256")
	}
	canonical, err := g.Canonical()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("global entity graph digest mismatch")
	}
	return nil
}

func canonicalNode(node Node) (Node, error) {
	node.NetworkID = strings.ToLower(strings.TrimSpace(node.NetworkID))
	node.Kind = strings.ToLower(strings.TrimSpace(node.Kind))
	node.Ref = strings.TrimSpace(node.Ref)
	if node.EvidenceState != securityevidence.StateObserved && node.EvidenceState != securityevidence.StateVerified {
		return Node{}, errors.New("entity node requires observed or verified state")
	}
	expectedID, err := StableNodeID(node.NetworkID, node.Kind, node.Ref)
	if err != nil {
		return Node{}, err
	}
	if node.ID == "" {
		node.ID = expectedID
	}
	if node.ID != expectedID {
		return Node{}, errors.New("entity id does not match canonical identity")
	}
	node.SourceEvents, err = canonicalDigests(node.SourceEvents)
	if err != nil {
		return Node{}, err
	}
	if len(node.SourceEvents) == 0 {
		return Node{}, errors.New("entity node requires source event")
	}
	return node, nil
}

func canonicalEdge(edge Edge) (Edge, error) {
	edge.SourceID = strings.TrimSpace(edge.SourceID)
	edge.TargetID = strings.TrimSpace(edge.TargetID)
	edge.Relation = strings.ToLower(strings.TrimSpace(edge.Relation))
	edge.Basis = strings.ToUpper(strings.TrimSpace(edge.Basis))
	edge.HypothesisReason = strings.TrimSpace(edge.HypothesisReason)
	edge.ID = ""

	if edge.SourceID == "" || edge.TargetID == "" || edge.SourceID == edge.TargetID {
		return Edge{}, errors.New("edge requires two distinct entities")
	}
	if !validRelation(edge.Relation) {
		return Edge{}, fmt.Errorf("unsupported entity relation %q", edge.Relation)
	}
	if edge.Relation == RelationSuspectedSameActor && edge.Basis != BasisHypothesis {
		return Edge{}, errors.New("suspected_same_actor must remain a hypothesis")
	}
	if edge.ObservedAtUnixMS <= 0 {
		return Edge{}, errors.New("edge observation time is required")
	}
	var err error
	edge.SourceEvents, err = canonicalDigests(edge.SourceEvents)
	if err != nil {
		return Edge{}, err
	}
	if len(edge.SourceEvents) == 0 {
		return Edge{}, errors.New("edge requires source event evidence")
	}

	switch edge.Basis {
	case BasisEvidence:
		if edge.EvidenceState != securityevidence.StateObserved && edge.EvidenceState != securityevidence.StateVerified {
			return Edge{}, errors.New("evidence edge requires observed or verified state")
		}
		if edge.ConfidenceBPS != 0 || edge.HypothesisReason != "" {
			return Edge{}, errors.New("evidence edge cannot carry hypothesis confidence")
		}
	case BasisHypothesis:
		if edge.EvidenceState != "" {
			return Edge{}, errors.New("hypothesis cannot claim evidence state")
		}
		if edge.ConfidenceBPS <= 0 || edge.ConfidenceBPS >= 10000 {
			return Edge{}, errors.New("hypothesis confidence must be between 1 and 9999 bps")
		}
		if edge.HypothesisReason == "" {
			return Edge{}, errors.New("hypothesis reason is required")
		}
	default:
		return Edge{}, errors.New("edge basis must be EVIDENCE or HYPOTHESIS")
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{
		edge.SourceID,
		edge.Relation,
		edge.TargetID,
		edge.Basis,
		string(edge.EvidenceState),
		fmt.Sprintf("%d", edge.ConfidenceBPS),
		edge.HypothesisReason,
		fmt.Sprintf("%d", edge.ObservedAtUnixMS),
		strings.Join(edge.SourceEvents, ","),
	}, "\x00")))
	edge.ID = "edge:" + hex.EncodeToString(sum[:])
	return edge, nil
}

func validNodeKind(kind string) bool {
	switch kind {
	case "network", "block", "transaction", "address", "account", "wallet", "contract", "program", "asset", "token", "pool", "bridge", "validator", "node", "miner", "protocol":
		return true
	default:
		return false
	}
}

func validRelation(relation string) bool {
	switch relation {
	case RelationFunded, RelationTransferredTo, RelationDrainedTo, RelationSwappedTo, RelationCalled, RelationCreated, RelationDeployed, RelationBridgedTo, RelationLiquidityTo, RelationHolds, RelationInteractedWith, RelationCorrelatedWith, RelationSuspectedSameActor:
		return true
	default:
		return false
	}
}

func crossChainRelationAllowed(relation string) bool {
	switch relation {
	case RelationBridgedTo, RelationCorrelatedWith, RelationSuspectedSameActor:
		return true
	default:
		return false
	}
}

func canonicalDigests(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !validSHA256(value) {
			return nil, fmt.Errorf("invalid source event digest %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func mergeDigests(left, right []string) []string {
	merged := append(append([]string(nil), left...), right...)
	canonical, _ := canonicalDigests(merged)
	return canonical
}

func evidenceRank(state securityevidence.EvidenceState) int {
	switch state {
	case securityevidence.StateVerified:
		return 2
	case securityevidence.StateObserved:
		return 1
	default:
		return 0
	}
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
