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
	core.Bundle = services.EvidenceBackedSecurityRadarBundle(core.Analysis.Bundle)
	core.Arms = services.ArvisArmsFromBundle(core.Bundle)
	if len(core.Arms) == 0 {
		core.Arms = core.Analysis.Arms
	}
	core.Final = services.ArvisFinalFromBundle(core.Bundle)
	return requestScope
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
