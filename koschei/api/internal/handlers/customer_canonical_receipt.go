package handlers

import (
	"strings"

	"koschei/api/internal/services"
)

// customerCanonicalMatchingReceipt links the exact cryptographically verified
// customer-visible final to a durable, re-read unified verdict history row.
// A compact cache index is not a signed decision receipt. A matching Signed
// boolean, risk grade, or fingerprint alone cannot establish the linkage.
func customerCanonicalMatchingReceipt(
	final services.UnifiedRadarVerdict,
	history []services.UnifiedRadarVerdictHistoryRecord,
	network, target string,
) (services.UnifiedRadarVerdictHistoryRecord, bool) {
	if services.VerifyUnifiedRadarVerdictWithCurrentSigner(final) != nil ||
		strings.TrimSpace(final.Target) != strings.TrimSpace(target) ||
		strings.TrimSpace(final.Network) != strings.TrimSpace(network) {
		return services.UnifiedRadarVerdictHistoryRecord{}, false
	}
	for _, record := range history {
		if strings.TrimSpace(record.ID) == "" ||
			strings.TrimSpace(record.LedgerID) == "" ||
			strings.TrimSpace(record.Fingerprint) == "" ||
			!record.Signed ||
			record.Network != final.Network ||
			record.TargetKind != "token" ||
			record.TargetID != final.Target ||
			record.Grade != final.Grade ||
			record.Verdict != final.Verdict ||
			record.RulesetVersion != final.RulesetVersion ||
			record.ActorRulesetVersion != final.ActorRuleset ||
			record.Digest != final.Digest ||
			record.Signature != final.Signature ||
			record.SignatureAlgorithm != final.SignatureAlgorithm ||
			record.KeyID != final.KeyID ||
			record.PayloadHash != final.PayloadHash {
			continue
		}
		return record, true
	}
	return services.UnifiedRadarVerdictHistoryRecord{}, false
}

// customerCanonicalReceiptGate is the billing readiness contract for the
// customer manual radar. Signed evidence without a durable exact-match receipt
// is useful for investigation but never counts as a finalized paid output.
func customerCanonicalReceiptGate(
	final services.UnifiedRadarVerdict,
	hasLiveEvidence bool,
	persistence string,
	history []services.UnifiedRadarVerdictHistoryRecord,
	network, target string,
) (string, services.UnifiedRadarVerdictHistoryRecord, bool) {
	status := customerInvestigationStatus(final, hasLiveEvidence)
	if status != "ready" {
		return status, services.UnifiedRadarVerdictHistoryRecord{}, false
	}
	record, matched := customerCanonicalMatchingReceipt(final, history, network, target)
	if persistence != "persisted" || !matched {
		return "persistence_pending", services.UnifiedRadarVerdictHistoryRecord{}, false
	}
	return "ready", record, true
}
