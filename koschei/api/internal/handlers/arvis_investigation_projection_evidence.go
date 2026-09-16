package handlers

import (
	"fmt"
	"strings"
)

// enrichArvisInvestigationProjection copies concrete identifiers that already
// exist in the unified investigation report into the fourteen customer-facing
// investigation results. It performs no network I/O, attribution inference or
// verdict changes.
func enrichArvisInvestigationProjection(projection map[string]any, report map[string]any) map[string]any {
	if projection == nil {
		projection = map[string]any{}
	}
	results, ok := projection["results"].([]arvisInvestigationResult)
	if !ok || len(results) == 0 {
		return projection
	}
	root := normalizeProjectionMap(report)
	actor := projectionMap(root["actor_investigation"])
	lifecycle := projectionMap(actor["token_lifecycle_recurrence"])
	exit := projectionMap(actor["exit_event_recurrence"])
	operational := projectionMap(actor["operational_memory"])
	funding := projectionMap(actor["funding_origin"])
	cluster := projectionMap(root["holder_cluster"])
	launch := projectionMap(root["launch_forensics"])
	flow := projectionMap(cluster["flow"])
	transactions := projectionSlice(root["transaction_evidence"])
	creator := projectionString(actor["wallet"])

	for index := range results {
		result := &results[index]
		if creator != "" {
			result.Actors = appendUniqueProjectionStrings(result.Actors, creator)
			result.Wallets = appendUniqueProjectionStrings(result.Wallets, creator)
		}
		switch result.ID {
		case "02", "03", "09":
			result.Tokens = appendUniqueProjectionStrings(result.Tokens, projectionStrings(lifecycle["other_mints"])...)
			result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionStrings(lifecycle["creation_signatures"])...)
			result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionStrings(lifecycle["creation_signatures"])...)
		case "04":
			result.Tokens = appendUniqueProjectionStrings(result.Tokens, projectionStrings(exit["other_targets"])...)
			result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionStrings(exit["signatures"])...)
			result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionStrings(exit["signatures"])...)
			for _, raw := range projectionSlice(exit["events"]) {
				item := projectionMap(raw)
				result.Tokens = appendUniqueProjectionStrings(result.Tokens, projectionString(item["target"]))
				result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionString(item["signature"]))
				result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionString(item["source_rule_id"]), projectionString(item["signature"]))
			}
		case "05":
			enrichSybilInvestigationResult(result, cluster, launch, flow)
		case "06":
			enrichFundingInvestigationResult(result, funding, cluster)
		case "07":
			enrichMoneyFlowInvestigationResult(result, creator, cluster, flow, transactions)
		case "13":
			enrichInfrastructureInvestigationResult(result, flow, transactions)
		case "08":
			for _, raw := range projectionSlice(operational["matches"]) {
				item := projectionMap(raw)
				wallet := projectionFirstString(item, "wallet", "candidate_wallet", "counterparty", "related_wallet", "actor_wallet")
				result.Wallets = appendUniqueProjectionStrings(result.Wallets, wallet)
				result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, wallet)
				result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionStrings(item["evidence_keys"])...)
			}
		}
	}
	projection["results"] = results
	return projection
}

func enrichSybilInvestigationResult(result *arvisInvestigationResult, cluster, launch, flow map[string]any) {
	if result == nil {
		return
	}
	associated := []string{}
	for _, key := range []string{"shared_funding_groups", "common_exit_groups"} {
		groups := projectionSlice(cluster[key])
		if key == "common_exit_groups" && len(groups) == 0 {
			groups = projectionSlice(flow[key])
		}
		for _, raw := range groups {
			group := projectionMap(raw)
			wallets := projectionStrings(group["wallets"])
			associated = appendUniqueProjectionStrings(associated, wallets...)
			result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionStrings(group["evidence"])...)
		}
	}
	associated = appendUniqueProjectionStrings(associated, projectionStrings(cluster["synchronized_wallets"])...)
	associated = appendUniqueProjectionStrings(associated, projectionStrings(flow["linked_wallets"])...)
	associated = appendUniqueProjectionStrings(associated, projectionStrings(flow["circular_wallets"])...)
	for _, raw := range projectionSlice(launch["profiles"]) {
		profile := projectionMap(raw)
		if projectionBool(profile["creator_linked"]) {
			associated = appendUniqueProjectionStrings(associated, projectionString(profile["owner_wallet"]))
		}
	}
	for _, raw := range projectionSlice(flow["observations"]) {
		item := projectionMap(raw)
		if projectionFirstString(item, "kind") == "holder_to_holder" || projectionFirstString(item, "kind") == "holder_to_holder_inbound_context" {
			associated = appendUniqueProjectionStrings(associated, projectionString(item["source_wallet"]), projectionString(item["destination"]))
			result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionString(item["signature"]))
			result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, projectionString(item["signature"]))
		}
	}
	result.Wallets = appendUniqueProjectionStrings(result.Wallets, associated...)
	result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, associated...)
	if len(associated) > 0 {
		result.Facts = appendUniqueProjectionStrings(result.Facts, fmt.Sprintf("Evidence-linked associated wallet observations: %d", len(associated)))
		result.CurrentFindings = appendUniqueProjectionStrings(result.CurrentFindings, "Associated wallets are listed only where shared-funding, synchronized activity, direct flow, circular flow, common-exit, or creator-linked launch evidence exists.")
		result.Answer = fmt.Sprintf("%d wallet(s) are linked by concrete campaign evidence in the collected window; these relations do not by themselves prove common real-world ownership.", len(associated))
	}
}

func enrichFundingInvestigationResult(result *arvisInvestigationResult, funding, cluster map[string]any) {
	if result == nil {
		return
	}
	source := projectionFirstString(funding, "source_wallet", "funding_wallet", "counterparty")
	destination := projectionString(funding["destination_wallet"])
	signature := projectionFirstString(funding, "signature", "transaction_signature")
	verification := projectionString(funding["verification_status"])
	amount := projectionFloat(funding["amount_sol"])
	slot := projectionInt(funding["slot"])

	result.Wallets = appendUniqueProjectionStrings(result.Wallets, source, destination)
	result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, source)
	result.Transactions = appendUniqueProjectionStrings(result.Transactions, signature)
	result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, signature)
	result.Entities = appendUniqueProjectionStrings(result.Entities,
		projectionString(funding["entity"]),
		projectionString(funding["entity_label"]),
		projectionString(funding["provider"]),
	)
	if source != "" {
		finding := "Observed funding source wallet: " + source
		if destination != "" {
			finding += " -> " + destination
		}
		if amount > 0 {
			finding += fmt.Sprintf(" (%.9f SOL)", amount)
		}
		if signature != "" {
			finding += "; tx=" + signature
		}
		if slot > 0 {
			finding += fmt.Sprintf("; slot=%d", slot)
		}
		if verification != "" {
			finding += "; verification=" + verification
		}
		result.CurrentFindings = appendUniqueProjectionStrings(result.CurrentFindings, finding)
		result.Answer = "On-chain funding evidence identifies " + source + " as a funding counterparty in the scanned history; identity beyond the wallet is not inferred."
	}

	for _, raw := range projectionSlice(cluster["wallets"]) {
		wallet := projectionMap(raw)
		holderWallet := projectionString(wallet["wallet"])
		fundingSource := projectionString(wallet["funding_source"])
		if holderWallet == "" || fundingSource == "" {
			continue
		}
		result.Wallets = appendUniqueProjectionStrings(result.Wallets, holderWallet, fundingSource)
		result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, fundingSource)
		result.CurrentFindings = appendUniqueProjectionStrings(result.CurrentFindings, fundingSource+" -> "+holderWallet+" (holder funding relation)")
	}
}

func enrichMoneyFlowInvestigationResult(result *arvisInvestigationResult, creator string, cluster, flow map[string]any, transactions []any) {
	if result == nil {
		return
	}
	observations := append([]any{}, projectionSlice(flow["observations"])...)
	observations = append(observations, projectionSlice(flow["internal_transfers"])...)
	if len(observations) == 0 {
		for _, raw := range projectionSlice(cluster["wallets"]) {
			wallet := projectionMap(raw)
			observations = append(observations, projectionSlice(wallet["flow_observations"])...)
		}
	}
	for _, raw := range observations {
		item := projectionMap(raw)
		source := projectionString(item["source_wallet"])
		destination := projectionString(item["destination"])
		signature := projectionString(item["signature"])
		mint := projectionString(item["mint"])
		kind := projectionString(item["kind"])
		direction := projectionString(item["direction"])
		amount := projectionFloat(item["amount"])
		result.Wallets = appendUniqueProjectionStrings(result.Wallets, source, destination)
		result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, source, destination)
		result.Transactions = appendUniqueProjectionStrings(result.Transactions, signature)
		result.Tokens = appendUniqueProjectionStrings(result.Tokens, mint)
		result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, signature)
		result.Entities = appendUniqueProjectionStrings(result.Entities,
			projectionString(item["from_entity"]), projectionString(item["to_entity"]),
		)
		result.Entities = appendUniqueProjectionStrings(result.Entities, projectionStrings(item["program_ids"])...)
		if source != "" || destination != "" {
			line := source + " -> " + destination
			if amount > 0 {
				line += fmt.Sprintf(" amount=%.9f", amount)
			}
			if direction != "" {
				line += " direction=" + direction
			}
			if kind != "" {
				line += " kind=" + kind
			}
			if signature != "" {
				line += " tx=" + signature
			}
			result.CurrentFindings = appendUniqueProjectionStrings(result.CurrentFindings, line)
		}
	}
	for _, raw := range transactions {
		item := projectionMap(raw)
		signature := projectionFirstString(item, "signature", "tx_signature", "transaction_signature")
		result.Transactions = appendUniqueProjectionStrings(result.Transactions, signature)
		result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, signature, projectionFirstString(item, "evidence_key", "reference"))
		trader := projectionString(item["trader"])
		result.Wallets = appendUniqueProjectionStrings(result.Wallets, trader)
		if trader != "" && trader != creator {
			result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, trader)
		}
	}
	if len(observations) > 0 {
		result.Facts = appendUniqueProjectionStrings(result.Facts, fmt.Sprintf("Direct holder-flow observations with source/destination context: %d", len(observations)))
		result.Answer = fmt.Sprintf("%d direct token-flow observation(s) expose source/destination wallet context; transaction-only rows remain supplemental when no recipient owner is resolved.", len(observations))
	}
}

func enrichInfrastructureInvestigationResult(result *arvisInvestigationResult, flow map[string]any, transactions []any) {
	if result == nil {
		return
	}
	for _, raw := range projectionSlice(flow["observations"]) {
		item := projectionMap(raw)
		result.Entities = appendUniqueProjectionStrings(result.Entities,
			projectionString(item["from_entity"]), projectionString(item["to_entity"]),
			projectionString(item["from_entity_category"]), projectionString(item["to_entity_category"]),
		)
		result.Entities = appendUniqueProjectionStrings(result.Entities, projectionStrings(item["program_ids"])...)
		result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionString(item["signature"]))
		result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs,
			projectionString(item["signature"]), projectionString(item["risk_flag_address"]), projectionString(item["risk_flag_endpoint"]),
		)
	}
	for _, raw := range transactions {
		item := projectionMap(raw)
		result.Transactions = appendUniqueProjectionStrings(result.Transactions, projectionFirstString(item, "signature", "tx_signature", "transaction_signature"))
		result.Entities = appendUniqueProjectionStrings(result.Entities,
			projectionString(item["source"]), projectionString(item["provider"]), projectionString(item["venue"]), projectionString(item["dex"]), projectionString(item["cex"]),
		)
	}
	if len(result.Entities) > 0 {
		result.Facts = appendUniqueProjectionStrings(result.Facts, fmt.Sprintf("Observed infrastructure/entity references: %d", len(result.Entities)))
	}
}

func projectionFirstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(projectionString(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func projectionBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func appendUniqueProjectionStrings(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	out := make([]string, 0, len(dst)+len(values))
	for _, value := range dst {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
