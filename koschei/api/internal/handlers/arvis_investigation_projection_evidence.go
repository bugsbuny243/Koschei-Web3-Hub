package handlers

import "strings"

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
		case "06":
			result.Counterparties = appendUniqueProjectionStrings(result.Counterparties,
				projectionString(funding["source_wallet"]),
				projectionString(funding["funding_wallet"]),
				projectionString(funding["counterparty"]),
			)
			result.Entities = appendUniqueProjectionStrings(result.Entities,
				projectionString(funding["entity"]),
				projectionString(funding["entity_label"]),
				projectionString(funding["provider"]),
			)
		case "07", "13":
			for _, raw := range transactions {
				item := projectionMap(raw)
				signature := projectionFirstString(item, "signature", "tx_signature", "transaction_signature")
				result.Transactions = appendUniqueProjectionStrings(result.Transactions, signature)
				result.EvidenceRefs = appendUniqueProjectionStrings(result.EvidenceRefs, signature, projectionFirstString(item, "evidence_key", "reference"))
				for _, key := range []string{"from", "from_wallet", "source_wallet", "sender", "to", "to_wallet", "destination_wallet", "recipient", "counterparty"} {
					value := projectionString(item[key])
					result.Wallets = appendUniqueProjectionStrings(result.Wallets, value)
					result.Counterparties = appendUniqueProjectionStrings(result.Counterparties, value)
				}
				for _, key := range []string{"entity", "entity_label", "provider", "program", "program_label", "venue", "dex", "cex"} {
					result.Entities = appendUniqueProjectionStrings(result.Entities, projectionString(item[key]))
				}
			}
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

func projectionFirstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(projectionString(values[key])); value != "" {
			return value
		}
	}
	return ""
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
