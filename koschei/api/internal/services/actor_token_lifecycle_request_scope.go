package services

import "strings"

// BuildRequestScopeTokenLifecycleRecurrence derives creator-token recurrence
// only from lifecycle observations produced and canonically verified in the
// current investigation request. It does not read or write persistence.
func BuildRequestScopeTokenLifecycleRecurrence(actorWallet, network, currentMint string, observations []ActorTokenLifecycleObservation) ActorTokenLifecycleRecurrence {
	actorWallet = strings.TrimSpace(actorWallet)
	network = normalizeRadarNetwork(network)
	currentMint = strings.TrimSpace(currentMint)
	out := ActorTokenLifecycleRecurrence{
		Status: "not_investigated", EvidenceStatus: "not_investigated",
		ActorWallet: actorWallet, Network: network, CurrentMint: currentMint,
		OtherMints: []string{}, CreationSignatures: []string{}, CreationSlots: []int64{},
		RuggedStatus: "not_classified_by_lifecycle_table", Limitations: []string{},
	}
	if actorWallet == "" {
		out.Status = "unavailable"
		out.EvidenceStatus = "unavailable"
		out.Limitations = append(out.Limitations, "Creator wallet is unavailable for request-scope lifecycle recurrence.")
		return out
	}

	seenMints := map[string]bool{}
	allReferencesComplete := true
	for _, observation := range observations {
		mint := strings.TrimSpace(observation.Mint)
		if mint == "" || seenMints[mint] {
			continue
		}
		if strings.TrimSpace(observation.ActorWallet) != "" && !strings.EqualFold(strings.TrimSpace(observation.ActorWallet), actorWallet) {
			continue
		}
		seenMints[mint] = true
		out.TotalTokens++
		switch strings.TrimSpace(observation.FateStatus) {
		case ActorTokenFateActive:
			out.ActiveTokens++
		case ActorTokenFateInactiveOrDead:
			out.InactiveOrDeadTokens++
		}
		if mint == currentMint {
			continue
		}
		out.OtherMints = append(out.OtherMints, mint)
		signature := strings.TrimSpace(observation.CreationSignature)
		if signature != "" {
			out.CreationSignatures = append(out.CreationSignatures, signature)
		}
		if observation.CreationSlot > 0 {
			out.CreationSlots = append(out.CreationSlots, observation.CreationSlot)
		}
		if signature == "" || observation.CreationSlot <= 0 || observation.FirstObservedAt.IsZero() || observation.LastObservedAt.IsZero() {
			allReferencesComplete = false
		}
	}

	out.Available = true
	out.OtherMints = uniqueFundingStrings(out.OtherMints)
	out.CreationSignatures = uniqueFundingStrings(out.CreationSignatures)
	out.CreationSlots = uniquePositiveLifecycleSlots(out.CreationSlots)
	out.ReferencesComplete = len(out.OtherMints) > 0 && allReferencesComplete

	switch {
	case out.TotalTokens == 0:
		out.Status = "not_investigated"
		out.EvidenceStatus = "not_investigated"
	case out.TotalTokens == 1 || len(out.OtherMints) == 0:
		out.Status = "single_token_only"
		out.EvidenceStatus = "observed"
	case out.ReferencesComplete:
		out.Status = "verified_request_scope_recurrence"
		out.EvidenceStatus = "verified"
	default:
		out.Status = "observed_request_scope_recurrence"
		out.EvidenceStatus = "observed"
		out.Limitations = append(out.Limitations, "Multiple creator-linked lifecycle observations exist in this request, but at least one other mint lacks a creation signature, slot or observation timestamp; VERIFIED was withheld.")
	}
	out.Limitations = append(out.Limitations,
		"Request-scope recurrence uses only canonically verified creator-mint observations collected in this investigation; persistence is not required.",
		"Lifecycle fate records active vs inactive/dead observations; this evidence alone does not classify a token as rugged.",
	)
	return out
}
