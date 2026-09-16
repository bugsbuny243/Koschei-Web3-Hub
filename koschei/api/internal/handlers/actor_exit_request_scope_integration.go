package handlers

import (
	"strings"

	"koschei/api/internal/services"
)

// applyRequestScopeActorExitRecurrence restores cross-token exit-event evidence
// from transaction-referenced evidence already available in the current request.
// A stronger persistent VERIFIED recurrence is never downgraded. Raw event rows
// are not persisted by this request-scope path.
func applyRequestScopeActorExitRecurrence(core *holderIntelligenceCoreResult, current services.ActorExitRecurrence, evidence []services.ActorDefenseEvidenceRecord, creator, network, target string) services.ActorExitRecurrence {
	if core == nil || strings.TrimSpace(creator) == "" {
		return current
	}
	requestScope := services.BuildRequestScopeActorExitRecurrence(creator, network, target, evidence)
	if !preferRequestScopeActorExit(current, requestScope) {
		return current
	}
	core.Analysis = services.ApplyActorExitRecurrenceToAnalysis(core.Analysis, requestScope)
	markRequestScopeActorExitProvenance(&core.Analysis)
	core.Bundle = services.EvidenceBackedSecurityRadarBundle(core.Analysis.Bundle)
	core.Arms = services.ArvisArmsFromBundle(core.Bundle)
	if len(core.Arms) == 0 {
		core.Arms = core.Analysis.Arms
	}
	core.Final = services.ArvisFinalFromBundle(core.Bundle)
	return requestScope
}

func preferRequestScopeActorExit(current, requestScope services.ActorExitRecurrence) bool {
	if requestScope.DistinctTargetsWithEvents < 2 || len(requestScope.OtherTargets) == 0 || !requestScope.ReferencesComplete {
		return false
	}
	if requestScope.EvidenceStatus == "verified" {
		return current.EvidenceStatus != "verified" || requestScope.DistinctTargetsWithEvents > current.DistinctTargetsWithEvents
	}
	if current.EvidenceStatus == "verified" {
		return false
	}
	return requestScope.DistinctTargetsWithEvents > current.DistinctTargetsWithEvents
}

// markRequestScopeActorExitProvenance corrects the legacy persistence marker set
// by ApplyActorExitRecurrenceToAnalysis when the selected evidence came from the
// current request instead of the persistent event index.
func markRequestScopeActorExitProvenance(analysis *services.ArvisAnalysis) {
	if analysis == nil {
		return
	}
	arms := services.ArvisArmsFromBundle(analysis.Bundle)
	if len(arms) == 0 {
		arms = append([]services.SecurityRadarVerdict{}, analysis.Arms...)
	}
	for index := range arms {
		if arms[index].ModuleID != services.ModuleRepeatActorScan {
			continue
		}
		if arms[index].Signals == nil {
			arms[index].Signals = map[string]any{}
		}
		arms[index].Signals["persistent_exit_event_index"] = false
		arms[index].Signals["request_scope_actor_exit"] = true
		break
	}
	analysis.Arms = arms
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = arms
}
