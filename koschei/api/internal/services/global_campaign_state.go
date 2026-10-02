package services

import "fmt"

// GlobalCampaignTransition records a deterministic lifecycle change request.
// It does not grant incident, verdict, grade, or containment authority.
type GlobalCampaignTransition struct {
	From         GlobalCampaignState `json:"from"`
	To           GlobalCampaignState `json:"to"`
	ReasonCodes  []string            `json:"reason_codes"`
	EvidenceRefs []string            `json:"evidence_refs"`
}

func NormalizeGlobalCampaignTransition(in GlobalCampaignTransition) GlobalCampaignTransition {
	in.ReasonCodes = normalizeGlobalCampaignStrings(in.ReasonCodes)
	in.EvidenceRefs = normalizeGlobalCampaignStrings(in.EvidenceRefs)
	return in
}

func ValidateGlobalCampaignTransition(in GlobalCampaignTransition) error {
	in = NormalizeGlobalCampaignTransition(in)
	if !validGlobalCampaignState(in.From) || !validGlobalCampaignState(in.To) {
		return fmt.Errorf("invalid global campaign transition state")
	}
	if in.From == in.To {
		return fmt.Errorf("global campaign transition must change state")
	}
	if !globalCampaignTransitionAllowed(in.From, in.To) {
		return fmt.Errorf("global campaign transition %q -> %q is not allowed", in.From, in.To)
	}
	if len(in.ReasonCodes) == 0 {
		return fmt.Errorf("global campaign transition requires at least one reason code")
	}
	if len(in.EvidenceRefs) == 0 {
		return fmt.Errorf("global campaign transition requires at least one evidence reference")
	}
	return nil
}

func globalCampaignTransitionAllowed(from, to GlobalCampaignState) bool {
	switch from {
	case GlobalCampaignEmerging:
		return to == GlobalCampaignActive || to == GlobalCampaignDormant || to == GlobalCampaignClosed
	case GlobalCampaignActive:
		return to == GlobalCampaignEscalating || to == GlobalCampaignDegrading || to == GlobalCampaignDormant || to == GlobalCampaignClosed
	case GlobalCampaignEscalating:
		return to == GlobalCampaignActive || to == GlobalCampaignDegrading || to == GlobalCampaignDormant || to == GlobalCampaignClosed
	case GlobalCampaignDegrading:
		return to == GlobalCampaignActive || to == GlobalCampaignDormant || to == GlobalCampaignClosed
	case GlobalCampaignDormant:
		return to == GlobalCampaignReopened || to == GlobalCampaignClosed
	case GlobalCampaignClosed:
		return to == GlobalCampaignReopened
	case GlobalCampaignReopened:
		return to == GlobalCampaignActive || to == GlobalCampaignEscalating || to == GlobalCampaignDormant || to == GlobalCampaignClosed
	default:
		return false
	}
}

// ApplyGlobalCampaignTransition creates the next immutable campaign revision.
// The caller remains responsible for persisting the old and new revisions.
func ApplyGlobalCampaignTransition(current GlobalCampaign, transition GlobalCampaignTransition) (GlobalCampaign, error) {
	if err := ValidateGlobalCampaign(current); err != nil {
		return GlobalCampaign{}, fmt.Errorf("current global campaign is invalid: %w", err)
	}
	transition = NormalizeGlobalCampaignTransition(transition)
	if current.State != transition.From {
		return GlobalCampaign{}, fmt.Errorf("global campaign transition source %q does not match current state %q", transition.From, current.State)
	}
	if err := ValidateGlobalCampaignTransition(transition); err != nil {
		return GlobalCampaign{}, err
	}

	next := current
	next.Revision++
	next.State = transition.To
	next.TransitionReasonCodes = append([]string{}, transition.ReasonCodes...)
	next.TransitionEvidenceRefs = append([]string{}, transition.EvidenceRefs...)
	next = NewGlobalCampaign(next)
	if err := ValidateGlobalCampaign(next); err != nil {
		return GlobalCampaign{}, fmt.Errorf("next global campaign is invalid: %w", err)
	}
	return next, nil
}
