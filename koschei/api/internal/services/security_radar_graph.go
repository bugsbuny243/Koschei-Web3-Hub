package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type SecurityRadarGraphNode struct {
	ID        string         `json:"id"`
	VerdictID string         `json:"verdict_id"`
	NodeID    string         `json:"node_id"`
	NodeType  string         `json:"node_type"`
	Label     string         `json:"label"`
	Address   string         `json:"address"`
	RiskLevel string         `json:"risk_level"`
	Weight    int            `json:"weight"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

type SecurityRadarGraphEdge struct {
	ID         string         `json:"id"`
	VerdictID  string         `json:"verdict_id"`
	SourceNode string         `json:"source_node"`
	TargetNode string         `json:"target_node"`
	EdgeType   string         `json:"edge_type"`
	Label      string         `json:"label"`
	RiskLevel  string         `json:"risk_level"`
	Weight     int            `json:"weight"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

type SecurityRadarGraphResponse struct {
	OK        bool                     `json:"ok"`
	VerdictID string                   `json:"verdict_id"`
	ModuleID  string                   `json:"module_id"`
	Target    string                   `json:"target"`
	Empty     bool                     `json:"empty"`
	Message   string                   `json:"message,omitempty"`
	Nodes     []SecurityRadarGraphNode `json:"nodes"`
	Edges     []SecurityRadarGraphEdge `json:"edges"`
}

func (s *SecurityRadarStore) LatestGraphForTarget(ctx context.Context, target string, moduleID string) (SecurityRadarGraphResponse, error) {
	verdict, err := s.latestGraphVerdictForTarget(ctx, target, moduleID)
	if err != nil {
		if err == sql.ErrNoRows || isSecurityRadarMissingRelation(err) {
			return emptySecurityRadarGraph("", moduleID, strings.TrimSpace(target)), nil
		}
		return SecurityRadarGraphResponse{}, err
	}
	return s.graphForVerdictRecord(ctx, verdict)
}

func (s *SecurityRadarStore) GraphByVerdictID(ctx context.Context, verdictID string) (SecurityRadarGraphResponse, error) {
	verdict, err := s.graphVerdictByID(ctx, verdictID)
	if err != nil {
		if err == sql.ErrNoRows || isSecurityRadarMissingRelation(err) {
			return emptySecurityRadarGraph(strings.TrimSpace(verdictID), "", ""), nil
		}
		return SecurityRadarGraphResponse{}, err
	}
	return s.graphForVerdictRecord(ctx, verdict)
}

func (s *SecurityRadarStore) InsertGraphNode(ctx context.Context, node SecurityRadarGraphNode) error {
	if s == nil || s.DB == nil {
		return nil
	}
	metadata, _ := json.Marshal(nonNilMap(node.Metadata))
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO security_radar_graph_nodes (verdict_id,node_id,node_type,label,address,risk_level,weight,metadata,created_at)
		VALUES ($1::uuid,$2,$3,$4,NULLIF($5,''),$6,$7,$8::jsonb,now())
		ON CONFLICT (verdict_id,node_id) DO UPDATE SET
			node_type=EXCLUDED.node_type,
			label=EXCLUDED.label,
			address=EXCLUDED.address,
			risk_level=EXCLUDED.risk_level,
			weight=EXCLUDED.weight,
			metadata=EXCLUDED.metadata`, node.VerdictID, node.NodeID, node.NodeType, node.Label, node.Address, firstSecurityRadarString(node.RiskLevel, "unknown"), node.Weight, string(metadata))
	return err
}

func (s *SecurityRadarStore) InsertGraphEdge(ctx context.Context, edge SecurityRadarGraphEdge) error {
	if s == nil || s.DB == nil {
		return nil
	}
	metadata, _ := json.Marshal(nonNilMap(edge.Metadata))
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO security_radar_graph_edges (verdict_id,source_node,target_node,edge_type,label,risk_level,weight,metadata,created_at)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8::jsonb,now())`, edge.VerdictID, edge.SourceNode, edge.TargetNode, edge.EdgeType, edge.Label, firstSecurityRadarString(edge.RiskLevel, "unknown"), edge.Weight, string(metadata))
	return err
}

func (s *SecurityRadarStore) graphForVerdictRecord(ctx context.Context, verdict SecurityRadarVerdictRecord) (SecurityRadarGraphResponse, error) {
	if s == nil || s.DB == nil {
		return emptySecurityRadarGraph(verdict.ID, verdict.ModuleID, verdict.Target), nil
	}
	nodes, err := s.graphNodes(ctx, verdict.ID)
	if err != nil {
		if isSecurityRadarMissingRelation(err) {
			built := BuildGraphFromVerdict(verdict)
			return withGraphHeader(built, verdict), nil
		}
		return SecurityRadarGraphResponse{}, err
	}
	edges, err := s.graphEdges(ctx, verdict.ID)
	if err != nil {
		if isSecurityRadarMissingRelation(err) {
			built := BuildGraphFromVerdict(verdict)
			return withGraphHeader(built, verdict), nil
		}
		return SecurityRadarGraphResponse{}, err
	}
	if len(nodes) == 0 {
		built := BuildGraphFromVerdict(verdict)
		return withGraphHeader(built, verdict), nil
	}
	return SecurityRadarGraphResponse{OK: true, VerdictID: verdict.ID, ModuleID: verdict.ModuleID, Target: verdict.Target, Empty: false, Nodes: nodes, Edges: edges}, nil
}

func (s *SecurityRadarStore) graphNodes(ctx context.Context, verdictID string) ([]SecurityRadarGraphNode, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id::text, verdict_id::text, node_id, node_type, COALESCE(label,''), COALESCE(address,''), COALESCE(risk_level,'unknown'), COALESCE(weight,0), COALESCE(metadata,'{}'::jsonb), created_at
		FROM security_radar_graph_nodes
		WHERE verdict_id=$1::uuid
		ORDER BY weight DESC, created_at ASC`, verdictID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecurityRadarGraphNode{}
	for rows.Next() {
		var item SecurityRadarGraphNode
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.VerdictID, &item.NodeID, &item.NodeType, &item.Label, &item.Address, &item.RiskLevel, &item.Weight, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SecurityRadarStore) graphEdges(ctx context.Context, verdictID string) ([]SecurityRadarGraphEdge, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id::text, verdict_id::text, source_node, target_node, edge_type, COALESCE(label,''), COALESCE(risk_level,'unknown'), COALESCE(weight,0), COALESCE(metadata,'{}'::jsonb), created_at
		FROM security_radar_graph_edges
		WHERE verdict_id=$1::uuid
		ORDER BY weight DESC, created_at ASC`, verdictID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecurityRadarGraphEdge{}
	for rows.Next() {
		var item SecurityRadarGraphEdge
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.VerdictID, &item.SourceNode, &item.TargetNode, &item.EdgeType, &item.Label, &item.RiskLevel, &item.Weight, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SecurityRadarStore) graphVerdictByID(ctx context.Context, verdictID string) (SecurityRadarVerdictRecord, error) {
	if s == nil || s.DB == nil {
		return SecurityRadarVerdictRecord{}, sql.ErrNoRows
	}
	verdictID = strings.TrimSpace(verdictID)
	if verdictID == "" {
		return SecurityRadarVerdictRecord{}, sql.ErrNoRows
	}
	return s.scanGraphVerdict(ctx, `WHERE id=$1::uuid`, verdictID)
}

func (s *SecurityRadarStore) latestGraphVerdictForTarget(ctx context.Context, target string, moduleID string) (SecurityRadarVerdictRecord, error) {
	if s == nil || s.DB == nil {
		return SecurityRadarVerdictRecord{}, sql.ErrNoRows
	}
	target = strings.TrimSpace(target)
	moduleID = strings.TrimSpace(moduleID)
	if target == "" {
		return SecurityRadarVerdictRecord{}, sql.ErrNoRows
	}
	if moduleID != "" {
		return s.scanGraphVerdict(ctx, `WHERE lower(target)=lower($1) AND module_id=$2 ORDER BY created_at DESC LIMIT 1`, target, moduleID)
	}
	return s.scanGraphVerdict(ctx, `WHERE lower(target)=lower($1) ORDER BY created_at DESC LIMIT 1`, target)
}

func (s *SecurityRadarStore) scanGraphVerdict(ctx context.Context, clause string, args ...any) (SecurityRadarVerdictRecord, error) {
	query := `SELECT id::text, COALESCE(event_id::text,''), module_id, target, target_type, network, grade, risk_index, risk_level, verdict, recommendation, evidence, signals, rule_version, signed, COALESCE(signature,''), created_at FROM security_radar_verdicts ` + clause
	var item SecurityRadarVerdictRecord
	var evidenceRaw, signalsRaw []byte
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&item.ID, &item.EventID, &item.ModuleID, &item.Target, &item.TargetType, &item.Network, &item.Grade, &item.RiskIndex, &item.RiskLevel, &item.Verdict, &item.Recommendation, &evidenceRaw, &signalsRaw, &item.RuleVersion, &item.Signed, &item.Signature, &item.CreatedAt); err != nil {
		return SecurityRadarVerdictRecord{}, err
	}
	_ = json.Unmarshal(evidenceRaw, &item.Evidence)
	_ = json.Unmarshal(signalsRaw, &item.Signals)
	if item.Evidence == nil {
		item.Evidence = []string{}
	}
	if item.Signals == nil {
		item.Signals = map[string]any{}
	}
	return item, nil
}

func emptySecurityRadarGraph(verdictID, moduleID, target string) SecurityRadarGraphResponse {
	return SecurityRadarGraphResponse{OK: true, VerdictID: verdictID, ModuleID: moduleID, Target: target, Empty: true, Message: "No node graph evidence is available for this verdict yet.", Nodes: []SecurityRadarGraphNode{}, Edges: []SecurityRadarGraphEdge{}}
}

func withGraphHeader(graph SecurityRadarGraphResponse, verdict SecurityRadarVerdictRecord) SecurityRadarGraphResponse {
	graph.OK = true
	graph.VerdictID = verdict.ID
	graph.ModuleID = verdict.ModuleID
	graph.Target = verdict.Target
	if graph.Nodes == nil {
		graph.Nodes = []SecurityRadarGraphNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []SecurityRadarGraphEdge{}
	}
	if len(graph.Nodes) == 0 {
		graph.Empty = true
		graph.Message = "No node graph evidence is available for this verdict yet."
	}
	return graph
}
