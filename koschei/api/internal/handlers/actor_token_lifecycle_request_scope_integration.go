package handlers

import (
	"strings"

	"koschei/api/internal/services"
)

// applyRequestScopeActorLifecycleRecurrence restores lifecycle visibility from
// canonically verified creator-mint observations already present in the current
// investigation. External discovery and the actor evidence graph are both read;
// no additional RPC call or persistent-memory claim is created here.
func applyRequestScopeActorLifecycleRecurrence(core *holderIntelligenceCoreResult, current services.ActorTokenLifecycleRecurrence, external actorExternalDiscoveryRun, dossier services.ActorDefenseDossier, creator, network, target string) services.ActorTokenLifecycleRecurrence {
	if core == nil || strings.TrimSpace(creator) == "" {
		return current
	}
	observations := append([]services.ActorTokenLifecycleObservation{}, external.CreatedMintPortfolio.LifecycleObservations...)
	observations = append(observations, verifiedLifecycleObservationsFromActorGraph(dossier, creator)...)
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

// verifiedLifecycleObservationsFromActorGraph exposes only canonical creator→mint
// graph edges to lifecycle. OBSERVED/INFERRED edges, edges without transaction
// references, and non-creator relations are deliberately excluded.
func verifiedLifecycleObservationsFromActorGraph(dossier services.ActorDefenseDossier, creator string) []services.ActorTokenLifecycleObservation {
	creator = strings.TrimSpace(creator)
	if creator == "" {
		return []services.ActorTokenLifecycleObservation{}
	}
	graph := services.BuildActorEvidenceGraph(dossier)
	out := []services.ActorTokenLifecycleObservation{}
	for _, edge := range graph.Edges {
		if edge.VerificationStatus != "verified" || !strings.EqualFold(strings.TrimSpace(edge.Relation), "created_token") {
			continue
		}
		if strings.TrimSpace(edge.Source) != creator || strings.TrimSpace(edge.Target) == "" {
			continue
		}
		if strings.TrimSpace(edge.Signature) == "" || edge.Slot <= 0 || edge.ObservedAt.IsZero() {
			continue
		}
		out = append(out, services.ActorTokenLifecycleObservation{
			Network:           dossier.Network,
			ActorWallet:       creator,
			Mint:              strings.TrimSpace(edge.Target),
			CreationSignature: strings.TrimSpace(edge.Signature),
			CreationSlot:      edge.Slot,
			FirstObservedAt:   edge.ObservedAt.UTC(),
			LastObservedAt:    edge.ObservedAt.UTC(),
			ObservationCount:  1,
			LifecycleStatus:   "creator_relation_observed",
		})
	}
	return out
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
	if requestScope.TotalTokens == 0 {
		return false
	}
	// A verified current creator→mint relation must at least surface as a
	// single-token lifecycle observation instead of 0/not_investigated.
	if current.TotalTokens == 0 || current.Status == "not_investigated" || current.Status == "unavailable" {
		return true
	}
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
