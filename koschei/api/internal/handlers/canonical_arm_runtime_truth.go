package handlers

import "fmt"

// canonicalArmRuntimeTruth distinguishes the architectural presence of ARVIS
// modules from completed evidence collection. It only changes capability
// diagnostics, never evidence, grades, rule execution or signature material.
func canonicalArmRuntimeTruth(item canonicalCapabilityStatus, runtimeRaw any, moduleCount int) canonicalCapabilityStatus {
	runtime := canonicalMap(runtimeRaw)
	if len(runtime) == 0 {
		if moduleCount > 0 {
			item.Status = canonicalCapabilityPartial
			item.EvidenceBacked = false
			item.Limitations = append(item.Limitations, "ARVIS module list is present, but runtime investigation_coverage is missing; module count is not completed evidence.")
		}
		return item
	}
	total := canonicalInt(runtime["capability_total"])
	completed := canonicalInt(runtime["completed"])
	pending := canonicalInt(runtime["evidence_pending"])
	unavailable := canonicalInt(runtime["source_unavailable"])
	insufficient := canonicalInt(runtime["insufficient_evidence"])
	notApplicable := canonicalInt(runtime["not_applicable"])
	producing := canonicalInt(runtime["evidence_producing"])
	gaps := canonicalSlice(runtime["critical_gaps"])

	item.EvidenceBacked = producing > 0
	item.Source = "investigation_coverage + modules/evidence_arms"
	if total <= 0 || total != moduleCount || completed < 0 || pending < 0 ||
		unavailable < 0 || insufficient < 0 || notApplicable < 0 || producing < 0 ||
		producing > completed ||
		completed+pending+unavailable+insufficient+notApplicable != total {
		item.Status = canonicalCapabilityPartial
		item.Limitations = append(item.Limitations,
			"ARVIS runtime coverage totals are missing or inconsistent with the module list; do not infer 14 completed collectors.")
		return item
	}
	if pending > 0 || unavailable > 0 || insufficient > 0 || len(gaps) > 0 ||
		completed+notApplicable != total ||
		canonicalString(runtime["status"]) != "complete_investigation" {
		item.Status = canonicalCapabilityPartial
		item.Limitations = append(item.Limitations, fmt.Sprintf(
			"ARVIS runtime: %d/%d completed, %d pending, %d not applicable, %d source unavailable, %d insufficient, %d critical gaps. Signed verdict status is separate from collector coverage.",
			completed, total, pending, notApplicable, unavailable, insufficient, len(gaps)))
		return item
	}
	item.Status = canonicalCapabilityActive
	return item
}
