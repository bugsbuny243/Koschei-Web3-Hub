package services

import "strings"

func HardenUnifiedRadarBehavior(report UnifiedRadarBehaviorReport, verification CreatorSellVerification, cluster HolderClusterAnalysis) UnifiedRadarBehaviorReport {
	report.Evidence = []ActorDefenseEvidenceRecord{}
	report.TriggeredRuleCount = 0
	report.WatchFlagCount = 0
	for index := range report.Signals {
		signal := &report.Signals[index]
		switch signal.RuleID {
		case UnifiedRuleCreatorSellAcceleration:
			if signal.EvidenceStatus != "unverified" {
				signal.EvidenceStatus = "observed"
			}
			signal.Signatures = append([]string{}, verification.VerifiedSignatures...)
			if signal.Metrics == nil {
				signal.Metrics = map[string]any{}
			}
			signal.Metrics["ledger_candidate_signature_count"] = len(verification.CandidateSignatures)
			signal.Metrics["verified_sell_signature_count"] = len(verification.VerifiedSignatures)
			signal.Metrics["transactions_parsed"] = verification.TransactionsParsed
			signal.Limitations = append(signal.Limitations, verification.Limitations...)
			signal.Summary = strings.ReplaceAll(signal.Summary, "verified sells", "ledger-observed sells")
		case UnifiedRuleDominantHolderFirstExit:
			hardenUnifiedDominantHolderExit(signal, cluster)
		}
		if signal.Triggered {
			report.TriggeredRuleCount++
		}
		if signal.EvidenceStatus == "inferred" {
			report.WatchFlagCount++
		}
		if evidence, ok := canonicalUnifiedSignalEvidence(report.Mint, *signal); ok {
			report.Evidence = append(report.Evidence, evidence)
		}
	}
	return report
}

func hardenUnifiedDominantHolderExit(signal *UnifiedRadarSignal, cluster HolderClusterAnalysis) {
	if signal == nil || !signal.Triggered || len(signal.Signatures) == 0 {
		return
	}
	signature := strings.TrimSpace(signal.Signatures[0])
	for _, wallet := range cluster.Wallets {
		for _, observation := range wallet.FlowObservations {
			if strings.TrimSpace(observation.Signature) != signature {
				continue
			}
			program := firstUnifiedProgram(observation.ProgramIDs)
			if signal.Metrics == nil {
				signal.Metrics = map[string]any{}
			}
			signal.Metrics["source_wallet"] = strings.TrimSpace(observation.SourceWallet)
			signal.Metrics["destination_wallet"] = strings.TrimSpace(observation.Destination)
			signal.Metrics["program"] = program
			signal.Metrics["program_ids"] = append([]string{}, observation.ProgramIDs...)
			signal.Metrics["slot"] = observation.Slot
			signal.Metrics["amount"] = observation.Amount
			if strings.TrimSpace(observation.SourceWallet) == "" || strings.TrimSpace(observation.Destination) == "" || program == "" || observation.Slot <= 0 {
				signal.EvidenceStatus = "observed"
				signal.Limitations = append(signal.Limitations, "Exit imzası görüldü ancak source, destination, program veya slot kanıt satırı eksik olduğu için VERIFIED kullanılmadı.")
			} else {
				signal.EvidenceStatus = "verified"
			}
			return
		}
	}
	signal.EvidenceStatus = "observed"
	signal.Limitations = append(signal.Limitations, "Exit signature holder-flow observations içinde yeniden eşleştirilemediği için VERIFIED kullanılmadı.")
}

func canonicalUnifiedSignalEvidence(mint string, signal UnifiedRadarSignal) (ActorDefenseEvidenceRecord, bool) {
	if signal.RuleID != UnifiedRuleDominantHolderFirstExit || !signal.Triggered || signal.EvidenceStatus != "verified" || len(signal.Signatures) == 0 {
		return ActorDefenseEvidenceRecord{}, false
	}
	source := unifiedMetricString(signal.Metrics, "source_wallet")
	destination := unifiedMetricString(signal.Metrics, "destination_wallet")
	program := unifiedMetricString(signal.Metrics, "program")
	slot, slotOK := unifiedInt64(signal.Metrics["slot"])
	amount := unifiedMetricFloat(signal.Metrics, "amount")
	if source == "" || destination == "" || program == "" || !slotOK || slot <= 0 || signal.ObservedAt.IsZero() {
		return ActorDefenseEvidenceRecord{}, false
	}
	item := ActorDefenseEvidenceRecord{
		Network: "solana-mainnet", ActorWallet: source, CounterpartKind: "wallet", CounterpartID: destination,
		Relation: "dominant_holder_first_exit", VerificationStatus: "verified",
		EvidenceKey: signal.EvidenceKeys[0], Source: "unified_manual_radar_transaction",
		Signature: strings.TrimSpace(signal.Signatures[0]), Slot: slot, ObservedAt: signal.ObservedAt.UTC(),
		TokenMint: strings.TrimSpace(mint), TokenAmount: amount,
		Metadata: map[string]any{
			"actor_role": "dominant_holder", "source_wallet": source, "destination_wallet": destination,
			"program": program, "unified_rule_id": signal.RuleID, "scope": signal.Scope,
			"summary": signal.Summary, "manual_only": true, "metrics": signal.Metrics,
		},
	}
	if !BuildActorDefenseEvidenceLine(item).EvidenceLineComplete {
		return ActorDefenseEvidenceRecord{}, false
	}
	return item, true
}

func firstUnifiedProgram(values []string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func unifiedMetricString(metrics map[string]any, key string) string {
	if metrics == nil {
		return ""
	}
	return strings.TrimSpace(actorFundingString(metrics[key]))
}

func unifiedMetricFloat(metrics map[string]any, key string) float64 {
	if metrics == nil {
		return 0
	}
	switch value := metrics[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
