package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"koschei/api/internal/services"
)

const (
	canonicalCapabilityActive       = "active"
	canonicalCapabilityPartial      = "partial"
	canonicalCapabilityUnavailable  = "unavailable"
	canonicalCapabilityNotRequested = "not_requested"
)

type canonicalCapabilityStatus struct {
	Capability             string   `json:"capability"`
	Status                 string   `json:"status"`
	WiredToCanonicalRadar  bool     `json:"wired_to_canonical_radar"`
	RequiredForFullScan    bool     `json:"required_for_full_scan"`
	EvidenceBacked         bool     `json:"evidence_backed"`
	Source                 string   `json:"source,omitempty"`
	Limitations            []string `json:"limitations"`
}

type canonicalIntegrationCoverage struct {
	SchemaVersion          string                              `json:"schema_version"`
	OverallStatus          string                              `json:"overall_status"`
	LiveScanRequested      bool                                `json:"live_scan_requested"`
	Capabilities          map[string]canonicalCapabilityStatus `json:"capabilities"`
	RequiredCapabilityCount int                                `json:"required_capability_count"`
	ActiveRequiredCount    int                                 `json:"active_required_count"`
	PartialRequiredCount   int                                 `json:"partial_required_count"`
	UnavailableRequiredCount int                               `json:"unavailable_required_count"`
	NotRequestedRequiredCount int                              `json:"not_requested_required_count"`
	OrphanCapabilityCount int                                  `json:"orphan_capability_count"`
	OrphanCapabilities     []string                            `json:"orphan_capabilities"`
	Policy                 map[string]any                      `json:"policy"`
}

// attachCanonicalInvestigationDiagnostics is called on the exact canonical
// report before immutable snapshotting. It makes capability reachability and the
// persistent actor graph explicit, so a module cannot silently exist outside the
// production investigation path.
func attachCanonicalInvestigationDiagnostics(report map[string]any) {
	if report == nil {
		return
	}
	actor := canonicalMap(report["actor_investigation"])
	if len(actor) > 0 {
		if _, exists := actor["evidence_graph"]; !exists {
			if dossier, ok := canonicalActorDossier(actor["dossier"]); ok {
				actor["evidence_graph"] = services.BuildActorEvidenceGraph(dossier)
			}
		}
		report["actor_investigation"] = actor
	}
	report["capability_integration"] = buildCanonicalIntegrationCoverage(report)
}

func buildCanonicalIntegrationCoverage(report map[string]any) canonicalIntegrationCoverage {
	live := canonicalLiveScanRequested(report)
	out := canonicalIntegrationCoverage{
		SchemaVersion: "koschei-capability-integration-v1",
		OverallStatus: canonicalCapabilityActive,
		LiveScanRequested: live,
		Capabilities: map[string]canonicalCapabilityStatus{},
		OrphanCapabilities: []string{},
		Policy: map[string]any{
			"feature_without_trigger_is_orphan": true,
			"feature_without_evidence_persistence_is_partial": true,
			"feature_without_report_contract_is_partial": true,
			"full_scan_cannot_claim_complete_when_required_capability_is_unavailable": true,
			"safe_check_can_leave_live_capabilities_not_requested": true,
		},
	}

	put := func(key string, item canonicalCapabilityStatus) {
		if item.Limitations == nil {
			item.Limitations = []string{}
		}
		out.Capabilities[key] = item
		if !item.WiredToCanonicalRadar {
			out.OrphanCapabilities = append(out.OrphanCapabilities, key)
			out.OrphanCapabilityCount++
		}
		if !item.RequiredForFullScan {
			return
		}
		out.RequiredCapabilityCount++
		switch item.Status {
		case canonicalCapabilityActive:
			out.ActiveRequiredCount++
		case canonicalCapabilityPartial:
			out.PartialRequiredCount++
		case canonicalCapabilityNotRequested:
			out.NotRequestedRequiredCount++
		default:
			out.UnavailableRequiredCount++
		}
	}

	modules := canonicalSlice(report["modules"])
	put("arvis_14_arm_engine", canonicalStatusFromCount(
		"ARVIS 14-arm evidence engine", len(modules), 14, true, "modules/evidence_arms",
		"Canonical radar did not expose every architecture arm in this report.",
	))
	put("holder_intelligence", canonicalStatusFromObject("Owner-resolved holder intelligence", report["holder_intelligence"], true, "holder_intelligence"))
	put("holder_cluster", canonicalStatusFromObject("Holder cluster and flow intelligence", report["holder_cluster"], true, "holder_cluster"))
	put("launch_forensics", canonicalStatusFromObject("Launch forensics", report["launch_forensics"], true, "launch_forensics"))
	put("lp_control", canonicalStatusFromObject("Liquidity and LP control", report["lp_control"], true, "lp_control"))
	put("jupiter_market_context", canonicalStatusFromObject("Jupiter route and exit context", report["jupiter_market_context"], false, "jupiter_market_context"))
	put("threat_anticipation", canonicalStatusFromObject("Threat anticipation", report["threat_anticipation"], true, "threat_anticipation"))
	put("deterministic_unified_verdict", canonicalStatusFromVerdict(report["final_verdict"]))

	actor := canonicalMap(report["actor_investigation"])
	put("creator_target_transition", canonicalStatusFromRun("Mint to creator target transition", actor["current_creator_relation"], live, true, "actor_investigation.current_creator_relation"))
	put("solscan_actor_discovery", canonicalStatusFromRun("Solscan actor discovery and attribution", actor["external_discovery"], live, true, "actor_investigation.external_discovery"))
	external := canonicalMap(actor["external_discovery"])
	put("created_mint_portfolio", canonicalStatusFromRun("Creator created-mint portfolio discovery and RPC verification", external["created_mint_portfolio"], live, true, "actor_investigation.external_discovery.created_mint_portfolio"))
	put("funding_origin", canonicalStatusFromRun("Actor funding origin", actor["funding_origin"], live, true, "actor_investigation.funding_origin"))
	put("actor_live_transaction_evidence", canonicalStatusFromRun("Actor live transaction evidence", actor["actor_live_evidence"], live, true, "actor_investigation.actor_live_evidence"))
	put("initial_distribution", canonicalStatusFromRun("Mint-specific initial recipient investigation", actor["current_token_distribution"], live, true, "actor_investigation.current_token_distribution"))
	put("persistent_actor_dossier", canonicalStatusFromDossier(actor["dossier"], live))
	put("actor_evidence_graph", canonicalStatusFromGraph(actor["evidence_graph"], live))
	put("actor_ruleset", canonicalStatusFromRun("Deterministic actor ruleset", actor["rule_verdict"], live, true, "actor_investigation.rule_verdict"))
	put("defense_agent_runtime", canonicalStatusFromRun("Solana Defense shadow runtime", report["defense_agent_runtime"], false, false, "defense_agent_runtime"))
	put("immutable_dossier_snapshot", canonicalCapabilityStatus{
		Capability: "Immutable dossier snapshot", Status: canonicalCapabilityActive,
		WiredToCanonicalRadar: true, RequiredForFullScan: false, EvidenceBacked: true,
		Source: "persistDossierSourceSnapshot", Limitations: []string{"Snapshot persistence remains best-effort when the database or signed verdict is unavailable."},
	})

	if !live {
		out.OverallStatus = "stored_or_preflight_projection"
		return out
	}
	switch {
	case out.UnavailableRequiredCount > 0 || out.OrphanCapabilityCount > 0:
		out.OverallStatus = "blocked"
	case out.PartialRequiredCount > 0 || out.NotRequestedRequiredCount > 0:
		out.OverallStatus = "partial"
	default:
		out.OverallStatus = "complete"
	}
	return out
}

func canonicalStatusFromCount(name string, observed, expected int, required bool, source, limitation string) canonicalCapabilityStatus {
	item := canonicalCapabilityStatus{Capability: name, WiredToCanonicalRadar: true, RequiredForFullScan: required, Source: source, Limitations: []string{}}
	switch {
	case observed >= expected:
		item.Status, item.EvidenceBacked = canonicalCapabilityActive, true
	case observed > 0:
		item.Status, item.EvidenceBacked = canonicalCapabilityPartial, true
		item.Limitations = append(item.Limitations, limitation)
	default:
		item.Status = canonicalCapabilityUnavailable
		item.Limitations = append(item.Limitations, limitation)
	}
	return item
}

func canonicalStatusFromObject(name string, raw any, required bool, source string) canonicalCapabilityStatus {
	item := canonicalCapabilityStatus{Capability: name, WiredToCanonicalRadar: true, RequiredForFullScan: required, Source: source, Limitations: []string{}}
	value := canonicalMap(raw)
	if len(value) == 0 {
		item.Status = canonicalCapabilityUnavailable
		item.Limitations = append(item.Limitations, "Canonical report section is absent or empty.")
		return item
	}
	status := strings.ToLower(strings.TrimSpace(canonicalString(value["status"])))
	available := canonicalBool(value["available"])
	item.Limitations = canonicalLimitations(value["limitations"])
	switch {
	case available || status == "complete" || status == "verified" || status == "evidence_backed" || status == "observed":
		item.Status, item.EvidenceBacked = canonicalCapabilityActive, true
	case strings.Contains(status, "partial") || strings.Contains(status, "bounded") || strings.Contains(status, "insufficient"):
		item.Status = canonicalCapabilityPartial
		item.EvidenceBacked = canonicalObjectHasEvidence(value)
	case strings.Contains(status, "not_requested"):
		item.Status = canonicalCapabilityNotRequested
	default:
		item.Status = canonicalCapabilityUnavailable
		item.EvidenceBacked = canonicalObjectHasEvidence(value)
	}
	return item
}

func canonicalStatusFromRun(name string, raw any, live, required bool, source string) canonicalCapabilityStatus {
	item := canonicalStatusFromObject(name, raw, required, source)
	item.WiredToCanonicalRadar = raw != nil
	if !live && (item.Status == canonicalCapabilityUnavailable || item.Status == canonicalCapabilityNotRequested) {
		item.Status = canonicalCapabilityNotRequested
		item.WiredToCanonicalRadar = true
	}
	return item
}

func canonicalStatusFromVerdict(raw any) canonicalCapabilityStatus {
	item := canonicalCapabilityStatus{Capability: "Deterministic Unified Verdict", WiredToCanonicalRadar: raw != nil, RequiredForFullScan: true, Source: "final_verdict", Limitations: []string{}}
	value := canonicalMap(raw)
	if canonicalBool(value["signed"]) && strings.TrimSpace(canonicalString(value["signature"])) != "" {
		item.Status, item.EvidenceBacked = canonicalCapabilityActive, true
		return item
	}
	item.Status = canonicalCapabilityUnavailable
	item.Limitations = append(item.Limitations, "Signed deterministic verdict is missing.")
	return item
}

func canonicalStatusFromDossier(raw any, live bool) canonicalCapabilityStatus {
	item := canonicalCapabilityStatus{Capability: "Persistent actor dossier", WiredToCanonicalRadar: raw != nil, RequiredForFullScan: true, Source: "actor_investigation.dossier", Limitations: []string{}}
	dossier, ok := canonicalActorDossier(raw)
	if !ok {
		if !live {
			item.Status = canonicalCapabilityNotRequested
			item.WiredToCanonicalRadar = true
		} else {
			item.Status = canonicalCapabilityUnavailable
			item.Limitations = append(item.Limitations, "Actor dossier could not be decoded.")
		}
		return item
	}
	item.EvidenceBacked = len(dossier.Evidence) > 0
	if item.EvidenceBacked || len(dossier.Tokens) > 0 || len(dossier.RelatedActors) > 0 {
		item.Status = canonicalCapabilityActive
	} else if live {
		item.Status = canonicalCapabilityPartial
		item.Limitations = append(item.Limitations, "Dossier is connected but no persistent actor relation was produced in this bounded run.")
	} else {
		item.Status = canonicalCapabilityNotRequested
	}
	return item
}

func canonicalStatusFromGraph(raw any, live bool) canonicalCapabilityStatus {
	item := canonicalCapabilityStatus{Capability: "Persistent actor evidence graph", WiredToCanonicalRadar: raw != nil, RequiredForFullScan: true, Source: "actor_investigation.evidence_graph", Limitations: []string{}}
	value := canonicalMap(raw)
	if canonicalBool(value["available"]) || canonicalInt(value["edge_count"]) > 0 {
		item.Status, item.EvidenceBacked = canonicalCapabilityActive, true
	} else if !live {
		item.Status = canonicalCapabilityNotRequested
		item.WiredToCanonicalRadar = true
	} else if raw != nil {
		item.Status = canonicalCapabilityPartial
		item.Limitations = append(item.Limitations, "Graph projector is connected but no evidence edge was available.")
	} else {
		item.Status = canonicalCapabilityUnavailable
	}
	return item
}

func canonicalLiveScanRequested(report map[string]any) bool {
	live := canonicalMap(report["full_scan_live_evidence"])
	status := strings.ToLower(strings.TrimSpace(canonicalString(live["status"])))
	return status != "" && status != "not_requested" && status != "stored_evidence_only" && status != "not_requested_preflight"
}

func canonicalActorDossier(raw any) (services.ActorDefenseDossier, bool) {
	if dossier, ok := raw.(services.ActorDefenseDossier); ok {
		return dossier, true
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) == 0 || string(encoded) == "null" {
		return services.ActorDefenseDossier{}, false
	}
	var dossier services.ActorDefenseDossier
	if json.Unmarshal(encoded, &dossier) != nil || strings.TrimSpace(dossier.Wallet) == "" {
		return services.ActorDefenseDossier{}, false
	}
	return dossier, true
}

func canonicalMap(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok && value != nil {
		return value
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) == 0 || string(encoded) == "null" {
		return map[string]any{}
	}
	value := map[string]any{}
	if json.Unmarshal(encoded, &value) != nil {
		return map[string]any{}
	}
	return value
}

func canonicalSlice(raw any) []any {
	if value, ok := raw.([]any); ok {
		return value
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) == 0 {
		return []any{}
	}
	value := []any{}
	if json.Unmarshal(encoded, &value) != nil {
		return []any{}
	}
	return value
}

func canonicalLimitations(raw any) []string {
	out := []string{}
	for _, item := range canonicalSlice(raw) {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" && value != "<nil>" {
			out = append(out, value)
		}
	}
	return out
}

func canonicalObjectHasEvidence(value map[string]any) bool {
	for _, key := range []string{"evidence", "findings", "transactions", "recipients", "candidates", "verified_candidates", "nodes", "edges"} {
		if len(canonicalSlice(value[key])) > 0 {
			return true
		}
	}
	for _, key := range []string{"evidence_persisted", "transactions_parsed", "parsed_evidence", "edge_count", "verified_evidence_count"} {
		if canonicalInt(value[key]) > 0 {
			return true
		}
	}
	return false
}

func canonicalString(raw any) string {
	if raw == nil {
		return ""
	}
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "<nil>" {
		return ""
	}
	return value
}

func canonicalBool(raw any) bool {
	if value, ok := raw.(bool); ok {
		return value
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(raw)), "true")
}

func canonicalInt(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		var parsed int
		_, _ = fmt.Sscan(strings.TrimSpace(fmt.Sprint(raw)), &parsed)
		return parsed
	}
}
