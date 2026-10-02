package services

import (
	"fmt"
	"strings"
)

// ValidateGlobalCampaign enforces the minimum identity/evidence boundary before
// a campaign revision may be persisted or exposed as materialized state.
func ValidateGlobalCampaign(c GlobalCampaign) error {
	if c.SchemaVersion != GlobalCampaignSchemaVersion {
		return fmt.Errorf("unexpected global campaign schema version %q", c.SchemaVersion)
	}
	if strings.TrimSpace(c.CampaignRef) == "" {
		return fmt.Errorf("global campaign reference is required")
	}
	if c.Revision == 0 {
		return fmt.Errorf("global campaign revision must be positive")
	}
	if !validGlobalCampaignState(c.State) {
		return fmt.Errorf("invalid global campaign state %q", c.State)
	}
	if c.VerifiedAnchorCount < 0 || c.ObservedAnchorCount < 0 {
		return fmt.Errorf("global campaign anchor counts cannot be negative")
	}
	if c.VerifiedAnchorCount+c.ObservedAnchorCount == 0 && !globalCampaignHasEvidenceRefs(c) {
		return fmt.Errorf("global campaign requires at least one evidence anchor or evidence reference")
	}
	if c.EvidenceHashSHA256 == "" || c.EvidenceHashSHA256 != hashGlobalCampaign(c) {
		return fmt.Errorf("global campaign evidence hash is missing or invalid")
	}
	if c.VerdictAuthority || c.GradeAuthority || c.ContainmentAuthority {
		return fmt.Errorf("global campaign cannot hold verdict, grade, or containment authority")
	}
	if c.SameOperatorClaim || c.RealWorldIdentityClaim || c.WrongdoingClaim {
		return fmt.Errorf("global campaign cannot make identity, common-control, or wrongdoing claims")
	}
	return nil
}

func globalCampaignHasEvidenceRefs(c GlobalCampaign) bool {
	return len(c.ObservationRefs) > 0 || len(c.RelationRefs) > 0 || len(c.BridgeLinkRefs) > 0 ||
		len(c.CampaignGenomeRefs) > 0 || len(c.CampaignTempoRefs) > 0 || len(c.BehaviorSignatureRefs) > 0 ||
		len(c.IncidentRefs) > 0 || len(c.VerdictRefs) > 0 || len(c.AttackPathRefs) > 0
}
