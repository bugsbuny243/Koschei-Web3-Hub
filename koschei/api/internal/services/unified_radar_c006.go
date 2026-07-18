package services

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	UnifiedRadarRulesetVersionV120 = "koschei-unified-radar-rules-v1.2.0"
)

type CrossTokenCreatorHolderTransfer struct {
	Available              bool                        `json:"available"`
	Status                 string                      `json:"status"`
	Mint                   string                      `json:"mint"`
	CreatorWallet          string                      `json:"creator_wallet"`
	RecipientTokenAccount  string                      `json:"recipient_token_account,omitempty"`
	RecipientOwnerWallet   string                      `json:"recipient_owner_wallet,omitempty"`
	RecipientOwnerResolved bool                        `json:"recipient_owner_resolved"`
	TransferSignature      string                      `json:"transfer_signature,omitempty"`
	Slot                   int64                       `json:"slot,omitempty"`
	Direction              string                      `json:"direction,omitempty"`
	Amount                 float64                     `json:"amount,omitempty"`
	Supply                 float64                     `json:"supply,omitempty"`
	SupplySharePct         float64                     `json:"supply_share_pct,omitempty"`
	OtherTokens            []RepeatDominantHolderMatch `json:"other_tokens"`
	ObservedAt             time.Time                   `json:"observed_at"`
	Limitations            []string                    `json:"limitations"`
}

func ApplyCrossTokenCreatorHolderTransferRuleV120(report UnifiedRadarBehaviorReport, relation CrossTokenCreatorHolderTransfer, now time.Time) UnifiedRadarBehaviorReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	if relation.ObservedAt.IsZero() {
		relation.ObservedAt = now
	}
	other := distinctC006OtherTokens(strings.TrimSpace(report.Mint), relation.OtherTokens)
	share := relation.SupplySharePct
	if share == 0 && relation.Supply > 0 && relation.Amount > 0 {
		share = roundUnifiedRadar((relation.Amount / relation.Supply) * 100)
	}

	signal := UnifiedRadarSignal{
		RuleID: UnifiedRuleCrossTokenCreatorHolderTransfer, Title: "Cross-token creator → dominant-holder transfer",
		EvidenceStatus: "unverified", Triggered: false, GradeEffect: "none",
		Scope: "parsed_creator_outbound_transfer_joined_to_persistent_actor_index",
		Metrics: map[string]any{
			"creator_wallet":          strings.TrimSpace(relation.CreatorWallet),
			"recipient_owner_wallet":  strings.TrimSpace(relation.RecipientOwnerWallet),
			"recipient_token_account": strings.TrimSpace(relation.RecipientTokenAccount),
			"transfer_signature":      strings.TrimSpace(relation.TransferSignature),
			"slot":                    relation.Slot, "direction": strings.TrimSpace(relation.Direction),
			"amount": relation.Amount, "source_wallet": strings.TrimSpace(relation.CreatorWallet), "destination_wallet": strings.TrimSpace(relation.RecipientOwnerWallet), "program": "spl-token", "supply_share_pct": share,
			"other_tokens": other,
		},
		Thresholds:   map[string]any{"evidence_window_days": RepeatDominantObservationDays, "holder_rank_max": 5, "d_cap_distinct_other_tokens": 2},
		EvidenceKeys: []string{}, Signatures: []string{}, Limitations: append([]string{}, relation.Limitations...), ObservedAt: relation.ObservedAt.UTC(),
	}
	creatorResolved := strings.TrimSpace(relation.CreatorWallet) != ""
	hasParsedSignature := strings.TrimSpace(relation.TransferSignature) != "" && relation.Slot > 0
	ownerResolved := relation.RecipientOwnerResolved && strings.TrimSpace(relation.RecipientOwnerWallet) != ""
	if !relation.Available || !creatorResolved {
		signal.Summary = "EVIDENCE PENDING: creator wallet transfer linkage was not evaluated because verified creator/deployer evidence is unavailable."
		signal.Limitations = append(signal.Limitations, "URD-C006 requires a VERIFIED creator/deployer wallet before cross-token transfer linkage can affect a grade.")
	} else if !hasParsedSignature {
		signal.Summary = "EVIDENCE PENDING: stored aggregates or incomplete transfer rows cannot trigger URD-C006 without a parsed transaction signature and slot."
		signal.Limitations = append(signal.Limitations, "No signature, no trigger; aggregate-only transfer evidence is watch/pending context.")
	} else if !ownerResolved {
		signal.Summary = "EVIDENCE PENDING: creator outbound transfer was parsed, but recipient owner resolution did not complete."
		signal.Limitations = append(signal.Limitations, "Recipient owner resolution is required before matching the transfer recipient to top-5 holder memory.")
		signal.Signatures = []string{strings.TrimSpace(relation.TransferSignature)}
	} else if len(other) == 0 {
		signal.EvidenceStatus = "verified"
		signal.Summary = "Creator outbound transfer was verified, but the recipient owner was not observed as a top-5 holder of another Koschei-observed token inside the evidence window."
		signal.Signatures = []string{strings.TrimSpace(relation.TransferSignature)}
		signal.EvidenceKeys = []string{"creator-holder-transfer:" + strings.TrimSpace(relation.TransferSignature)}
	} else {
		signal.EvidenceStatus = "verified"
		signal.Triggered = true
		signal.Signatures = []string{strings.TrimSpace(relation.TransferSignature)}
		signal.EvidenceKeys = []string{"creator-holder-transfer:" + strings.TrimSpace(relation.TransferSignature)}
		capGrade := "C"
		if len(other) >= 2 {
			capGrade = "D"
		}
		signal.GradeEffect = "hard_cap_" + capGrade
		signal.Summary = fmt.Sprintf("Creator wallet transferred %.4f%% of supply to a wallet observed as a dominant holder in %d other Koschei-observed token(s). This links launch and concentration across tokens; it is not a claim of identity, coordination intent, or wrongdoing.", share, len(other))
	}
	report.RulesetVersion = UnifiedRadarRulesetVersionV120
	report.Signals = append(report.Signals, signal)
	if signal.Triggered {
		report.TriggeredRuleCount++
	}
	if evidence, ok := unifiedSignalEvidence(report.CreatorWallet, report.Mint, signal); ok {
		report.Evidence = append(report.Evidence, evidence)
	}
	return report
}

func EvaluateUnifiedRadarVerdictV120(target string, actor ActorDefenseRuleVerdict, behavior UnifiedRadarBehaviorReport) UnifiedRadarVerdict {
	out := EvaluateUnifiedRadarVerdictV110(target, actor, behavior)
	out.RulesetVersion = UnifiedRadarRulesetVersionV120
	capGrade := ""
	for _, signal := range behavior.Signals {
		if signal.RuleID != UnifiedRuleCrossTokenCreatorHolderTransfer || !signal.Triggered || signal.EvidenceStatus != "verified" {
			continue
		}
		if signal.GradeEffect == "hard_cap_D" {
			capGrade = "D"
		} else if capGrade == "" && signal.GradeEffect == "hard_cap_C" {
			capGrade = "C"
		}
	}
	if capGrade != "" {
		out.Grade = worseUnifiedGrade(out.Grade, capGrade)
		out.Verdict = "hard_trigger"
		out.DecisionPath = append(out.DecisionPath, "URD-C006 fixed the maximum grade at "+capGrade+" from a VERIFIED parsed creator transfer joined to owner-resolved cross-token dominant-holder memory.")
	}
	out.Signed = out.Grade != "-" && len(out.TriggeredRules) > 0
	out.Signature = ""
	if out.Signed {
		out.Signature = signUnifiedRadarVerdict(strings.TrimSpace(target), out)
	}
	return out
}

func distinctC006OtherTokens(currentMint string, matches []RepeatDominantHolderMatch) []RepeatDominantHolderMatch {
	currentMint = strings.TrimSpace(currentMint)
	byMint := map[string]RepeatDominantHolderMatch{}
	for _, match := range matches {
		mint := strings.TrimSpace(match.Mint)
		if mint == "" || mint == currentMint || match.Rank < 1 || match.Rank > 5 {
			continue
		}
		if prev, ok := byMint[mint]; !ok || match.Percentage > prev.Percentage {
			match.Mint = mint
			byMint[mint] = match
		}
	}
	out := make([]RepeatDominantHolderMatch, 0, len(byMint))
	for _, match := range byMint {
		out = append(out, match)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Percentage == out[j].Percentage {
			return out[i].Mint < out[j].Mint
		}
		return out[i].Percentage > out[j].Percentage
	})
	return out
}
