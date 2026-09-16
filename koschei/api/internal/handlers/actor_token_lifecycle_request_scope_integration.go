package handlers

import (
	"strings"

	"koschei/api/internal/services"
)

// applyRequestScopeActorLifecycleRecurrence restores Repeat Actor Scan from
// canonically verified creator-mint lifecycle observations collected in the
// current live request when persistent lifecycle memory is unavailable or less
// complete. Persistent VERIFIED evidence remains preferred when it is at least
// as strong. This helper never changes deterministic verdict rules.
func applyRequestScopeActorLifecycleRecurrence(core *holderIntelligenceCoreResult, current services.ActorTokenLifecycleRecurrence, external actorExternalDiscoveryRun, creator, network, target string) services.ActorTokenLifecycleRecurrence {
	if core == nil || strings.TrimSpace(creator) == "" {
		return current
	}
	observations := append([]services.ActorTokenLifecycleObservation{}, external.CreatedMintPortfolio.LifecycleObservations...)
	requestScope := services.BuildRequestScopeTokenLifecycleRecurrence(creator, network, target, observations)
	if !preferRequestScopeActorLifecycle(current, requestScope) {
		return current
	}
	core.Analysis = services.ApplyActorTokenLifecycleRecurrenceToAnalysis(core.Analysis, requestScope)
	markRequestScopeActorLifecycleProvenance(&core.Analysis)
	core.Bundle = services.EvidenceBackedSecurityRadarBundle(core.Analysis.Bundle)
	core.Arms = services.ArvisArmsFromBundle(core.Bundle)
	if len(core.Arms) == 0 {
		core.Arms = core.Analysis.Arms
	}
	core.Final = services.ArvisFinalFromBundle(core.Bundle)
	return requestScope
}

// markRequestScopeActorLifecycleProvenance prevents live, request-scoped
// recurrence from masquerading as evidence loaded from the persistent actor
// lifecycle index. The recurrence evidence and VERIFIED state are unchanged;
// only its provenance markers are corrected.
func markRequestScopeActorLifecycleProvenance(analysis *services.ArvisAnalysis) {
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
		arms[index].Signals["persistent_actor_lifecycle_index"] = false
		arms[index].Signals["request_scope_actor_lifecycle"] = true
		break
	}
	analysis.Arms = arms
	if analysis.Bundle.Metadata == nil {
		analysis.Bundle.Metadata = map[string]any{}
	}
	analysis.Bundle.Metadata["arvis_arms"] = arms
}

func preferRequestScopeActorLifecycle(current, requestScope services.ActorTokenLifecycleRecurrence) bool {
	if requestScope.TotalTokens < 2 || len(requestScope.OtherMints) == 0 {
		return false
	}
	if requestScope.EvidenceStatus == "verified" {
		return current.EvidenceStatus != "verified" || requestScope.TotalTokens > current.TotalTokens
	}
	if current.EvidenceStatus == "verified" {
		return false
	}
	return requestScope.TotalTokens > current.TotalTokens
}
