package handlers

import "strings"

// attachCreatorCompatibilityFields exposes the already-collected canonical actor
// evidence under stable top-level keys. It performs no RPC or database work and
// does not create a second creator investigation path.
func attachCreatorCompatibilityFields(report map[string]any) {
	if report == nil || strings.EqualFold(strings.TrimSpace(dossierString(report["analysis_scope"])), "wallet_actor_investigation") {
		return
	}
	actor := dossierMap(report["actor_investigation"])
	wallet := strings.TrimSpace(dossierString(actor["wallet"]))
	integration := dossierMap(actor["integration_run"])
	status := strings.TrimSpace(dossierString(integration["status"]))
	if status == "" {
		if wallet == "" {
			status = "creator_wallet_not_observed"
		} else {
			status = "stored_evidence_only"
		}
	}
	report["creator_intelligence"] = map[string]any{
		"available":                  wallet != "",
		"status":                     status,
		"creator_wallet":             wallet,
		"store_status":               actor["store_status"],
		"dossier":                    actor["dossier"],
		"current_creator_relation":   actor["current_creator_relation"],
		"external_discovery":         actor["external_discovery"],
		"funding_origin":             actor["funding_origin"],
		"funding_origin_persistence": actor["funding_origin_persistence"],
		"live_evidence":              actor["actor_live_evidence"],
		"rule_verdict":               actor["rule_verdict"],
		"rule_verdict_persistence":   actor["rule_verdict_persistence"],
		"limitations":                integration["limitations"],
	}

	distribution := dossierMap(actor["current_token_distribution"])
	distributionStatus := strings.TrimSpace(dossierString(distribution["status"]))
	if distributionStatus == "" {
		distributionStatus = "not_requested"
	}
	distributionAvailable := wallet != "" && distributionStatus != "not_requested" &&
		distributionStatus != "persistence_unavailable" &&
		distributionStatus != "creator_mint_relation_unavailable" &&
		distributionStatus != "creator_mint_relation_unresolved"
	report["creator_distribution"] = map[string]any{
		"available":            distributionAvailable,
		"status":               distributionStatus,
		"creator_wallet":       wallet,
		"target":               distribution["target"],
		"report":               distribution["report"],
		"evidence_produced":    distribution["evidence_produced"],
		"evidence_persisted":   distribution["evidence_persisted"],
		"persistence_failures": distribution["persistence_failures"],
		"limitations":          distribution["limitations"],
	}
}
