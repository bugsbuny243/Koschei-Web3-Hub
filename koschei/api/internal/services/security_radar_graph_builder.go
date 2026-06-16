package services

import (
	"fmt"
	"strings"
	"time"
)

func BuildGraphFromVerdict(verdict SecurityRadarVerdictRecord) SecurityRadarGraphResponse {
	graph := emptySecurityRadarGraph(verdict.ID, verdict.ModuleID, verdict.Target)
	if verdict.ID == "" && verdict.Target == "" {
		return graph
	}
	now := time.Now().UTC()
	centerID := "target"
	centerType := centerNodeType(verdict.ModuleID)
	if !isLikelySolanaAddress(verdict.Target) && !strings.HasPrefix(strings.ToLower(verdict.Target), "http") {
		centerType = "unknown"
	}
	center := SecurityRadarGraphNode{VerdictID: verdict.ID, NodeID: centerID, NodeType: centerType, Label: centerLabel(verdict.ModuleID), Address: verdict.Target, RiskLevel: verdict.RiskLevel, Weight: maxGraphWeight(verdict.RiskIndex, 50), Metadata: map[string]any{"grade": verdict.Grade, "risk_index": verdict.RiskIndex, "source": "verdict"}, CreatedAt: now}

	addrs := collectGraphAddresses(verdict.Signals)
	nodes := []SecurityRadarGraphNode{center}
	edges := []SecurityRadarGraphEdge{}
	addGroup := func(key, nodeType, edgeType, label string) {
		values := addrs[key]
		for i, address := range values {
			if address == "" || address == verdict.Target {
				continue
			}
			nodeID := fmt.Sprintf("%s_%d", nodeType, i+1)
			nodes = append(nodes, SecurityRadarGraphNode{VerdictID: verdict.ID, NodeID: nodeID, NodeType: nodeType, Label: label, Address: address, RiskLevel: verdict.RiskLevel, Weight: graphWeightForNode(nodeType, verdict.RiskIndex), Metadata: map[string]any{"signal_key": key}, CreatedAt: now})
			edges = append(edges, SecurityRadarGraphEdge{VerdictID: verdict.ID, SourceNode: nodeID, TargetNode: centerID, EdgeType: edgeType, Label: label, RiskLevel: verdict.RiskLevel, Weight: graphWeightForNode(nodeType, verdict.RiskIndex), Metadata: map[string]any{"signal_key": key}, CreatedAt: now})
		}
	}

	switch verdict.ModuleID {
	case ModuleRaydiumPoolGuardian:
		addGroup("mint_authority", "creator_wallet", "authority_relation", "Mint authority")
		addGroup("freeze_authority", "creator_wallet", "authority_relation", "Freeze authority")
		addGroup("lp_wallets", "lp_wallet", "lp_relation", "LP wallet")
		addGroup("lp_wallet", "lp_wallet", "lp_relation", "LP wallet")
		addGroup("pool", "pool", "lp_relation", "Pool relation")
		addGroup("holder_wallets", "lp_wallet", "lp_relation", "Holder wallet")
	case ModuleWalletlessClaimShield:
		addGroup("claim_program", "claim_program", "claim_relation", "Claim program")
		addGroup("unsafe_programs", "claim_program", "claim_relation", "Unsafe program")
		addGroup("related_programs", "claim_program", "claim_relation", "Related program")
		addGroup("related_wallets", "unknown", "claim_relation", "Related wallet")
	default:
		addGroup("creator_wallet", "creator_wallet", "creator_link", "Creator wallet")
		addGroup("funding_sources", "funding_source", "funded", "Funding source")
		addGroup("funding_source", "funding_source", "funded", "Funding source")
		addGroup("early_buyers", "early_buyer", "bought_early", "Early buyer")
		addGroup("sniper_wallets", "sniper_wallet", "bought_early", "Sniper wallet")
		addGroup("exit_wallets", "exit_wallet", "exit_flow", "Exit wallet")
	}

	if len(nodes) <= 1 {
		return graph
	}
	return SecurityRadarGraphResponse{OK: true, VerdictID: verdict.ID, ModuleID: verdict.ModuleID, Target: verdict.Target, Empty: false, Nodes: nodes, Edges: edges}
}

func centerNodeType(moduleID string) string {
	switch moduleID {
	case ModuleRaydiumPoolGuardian:
		return "pool"
	case ModuleWalletlessClaimShield:
		return "claim_program"
	default:
		return "token"
	}
}

func centerLabel(moduleID string) string {
	switch moduleID {
	case ModuleRaydiumPoolGuardian:
		return "Raydium pool / token"
	case ModuleWalletlessClaimShield:
		return "Claim target"
	default:
		return "Target token"
	}
}

func collectGraphAddresses(signals map[string]any) map[string][]string {
	out := map[string][]string{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case string:
			if isLikelySolanaAddress(v) {
				out[prefix] = appendUniqueGraphAddress(out[prefix], strings.TrimSpace(v))
			}
		case []any:
			for _, item := range v {
				walk(prefix, item)
			}
		case map[string]any:
			for k, item := range v {
				key := strings.TrimSpace(k)
				if prefix != "" && !strings.Contains(prefix, key) {
					key = prefix + "." + key
				}
				walk(key, item)
			}
		}
	}
	for k, v := range signals {
		walk(strings.TrimSpace(k), v)
	}
	aliases := map[string][]string{
		"creator_wallet":   matchSignalAddresses(out, "creator"),
		"funding_sources":  matchSignalAddresses(out, "funding"),
		"funding_source":   matchSignalAddresses(out, "funder"),
		"early_buyers":     matchSignalAddresses(out, "early"),
		"sniper_wallets":   matchSignalAddresses(out, "sniper"),
		"exit_wallets":     matchSignalAddresses(out, "exit"),
		"mint_authority":   matchSignalAddresses(out, "mint_authority"),
		"freeze_authority": matchSignalAddresses(out, "freeze_authority"),
		"lp_wallets":       matchSignalAddresses(out, "lp"),
		"lp_wallet":        matchSignalAddresses(out, "lp_wallet"),
		"holder_wallets":   matchSignalAddresses(out, "holder"),
		"pool":             matchSignalAddresses(out, "pool"),
		"claim_program":    matchSignalAddresses(out, "claim"),
		"unsafe_programs":  matchSignalAddresses(out, "unsafe"),
		"related_programs": matchSignalAddresses(out, "program"),
		"related_wallets":  matchSignalAddresses(out, "related"),
	}
	for k, v := range aliases {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

func matchSignalAddresses(values map[string][]string, needle string) []string {
	result := []string{}
	needle = strings.ToLower(needle)
	for key, addresses := range values {
		if strings.Contains(strings.ToLower(key), needle) {
			for _, address := range addresses {
				result = appendUniqueGraphAddress(result, address)
			}
		}
	}
	return result
}

func appendUniqueGraphAddress(values []string, address string) []string {
	address = strings.TrimSpace(address)
	for _, existing := range values {
		if existing == address {
			return values
		}
	}
	return append(values, address)
}

func graphWeightForNode(nodeType string, riskIndex int) int {
	base := maxGraphWeight(riskIndex, 30)
	switch nodeType {
	case "funding_source", "sniper_wallet", "exit_wallet":
		return base + 12
	case "creator_wallet", "claim_program":
		return base + 8
	default:
		return base
	}
}

func maxGraphWeight(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}
