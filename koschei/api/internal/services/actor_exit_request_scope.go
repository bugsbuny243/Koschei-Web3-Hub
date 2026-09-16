package services

import (
	"sort"
	"strings"
)

// BuildRequestScopeActorExitRecurrence derives cross-token exit recurrence from
// transaction-referenced evidence already collected during the current request.
// It does not persist raw events and it never upgrades incomplete references to
// verified evidence.
func BuildRequestScopeActorExitRecurrence(actorWallet, network, currentTarget string, evidence []ActorDefenseEvidenceRecord) ActorExitRecurrence {
	actorWallet = strings.TrimSpace(actorWallet)
	network = normalizeRadarNetwork(network)
	currentTarget = strings.TrimSpace(currentTarget)
	out := ActorExitRecurrence{
		Status: "not_investigated", EvidenceStatus: "not_investigated",
		ActorWallet: actorWallet, Network: network, CurrentTarget: currentTarget,
		OtherTargets: []string{}, Signatures: []string{}, Slots: []int64{}, EventKinds: []string{}, Events: []ActorExitEventReference{}, Limitations: []string{
			"Request-scope exit recurrence uses transaction-referenced evidence collected during the current investigation only.",
			"Inactive or zero-liquidity lifecycle state alone is not classified as an exit or rug event.",
			"Raw event evidence is not persisted by this projection.",
		},
	}
	if actorWallet == "" {
		out.Status = "actor_unavailable"
		out.EvidenceStatus = "unavailable"
		return out
	}

	seen := map[string]struct{}{}
	targets := map[string]struct{}{}
	otherTargets := map[string]struct{}{}
	signatures := map[string]struct{}{}
	slots := map[int64]struct{}{}
	kinds := map[string]struct{}{}
	allVerified := true

	for _, item := range evidence {
		event, ok := actorExitEventFromEvidence(item)
		if !ok || !strings.EqualFold(strings.TrimSpace(event.ActorWallet), actorWallet) || normalizeRadarNetwork(event.Network) != network {
			continue
		}
		key := strings.Join([]string{event.Target, event.EventKind, event.Signature}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ref := ActorExitEventReference{
			Target: event.Target, EventKind: event.EventKind, EvidenceState: event.EvidenceState,
			Signature: event.Signature, Slot: event.Slot, ObservedAt: event.ObservedAt, SourceRuleID: event.SourceRuleID,
		}
		out.Events = append(out.Events, ref)
		targets[event.Target] = struct{}{}
		if event.Target != currentTarget {
			otherTargets[event.Target] = struct{}{}
		}
		signatures[event.Signature] = struct{}{}
		slots[event.Slot] = struct{}{}
		kinds[event.EventKind] = struct{}{}
		if event.EvidenceState != "verified" {
			allVerified = false
		}
	}

	out.DistinctTargetsWithEvents = len(targets)
	out.OtherTargets = sortedStringSet(otherTargets)
	out.Signatures = sortedStringSet(signatures)
	out.Slots = sortedInt64Set(slots)
	out.EventKinds = sortedStringSet(kinds)
	out.Available = len(out.Events) > 0
	out.ReferencesComplete = len(out.Events) > 0

	switch {
	case len(out.Events) == 0:
		out.Status = "no_transaction_referenced_exit_events"
		out.EvidenceStatus = "not_observed"
	case len(out.OtherTargets) == 0:
		out.Status = "single_target_only"
		if allVerified {
			out.EvidenceStatus = "verified"
		} else {
			out.EvidenceStatus = "observed"
		}
	case allVerified:
		out.Status = "verified_request_scope_exit_recurrence"
		out.EvidenceStatus = "verified"
	default:
		out.Status = "observed_request_scope_exit_recurrence"
		out.EvidenceStatus = "observed"
	}
	return out
}

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sortedInt64Set(values map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
