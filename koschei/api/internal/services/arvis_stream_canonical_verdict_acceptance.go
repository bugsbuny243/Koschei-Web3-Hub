package services

import (
	"errors"
	"fmt"
	"strings"
)

// validateArvisStreamCanonicalVerdict is a side-effect-free acceptance gate for
// a future stream-to-canonical adapter. It never issues a grade or signature.
// In particular, it cannot promote the unsigned ARVIS compatibility final.
//
// Authentication alone does not prove that the signed decision belongs to the
// source stream event. Require exact token/network identity and an on-chain
// signature cited by a VERIFIED deterministic rule in the signed payload.
// Event-ID/slot provenance and atomic persistence remain separate requirements.
func validateArvisStreamCanonicalVerdict(
	event arvisStreamTarget,
	verdict UnifiedRadarVerdict,
	registry GlobalRadarTrustedVerdictRegistry,
) error {
	target := strings.TrimSpace(event.Target)
	network := strings.TrimSpace(event.Network)
	signature := strings.TrimSpace(event.Signature)
	if strings.TrimSpace(event.ID) == "" || target == "" || signature == "" ||
		network != "solana-mainnet" {
		return errors.New("stream canonical verdict requires an identified Solana mainnet token event and transaction signature")
	}
	if event.ModuleID != ModulePumpSybilRadar && event.ModuleID != ModuleRaydiumPoolGuardian {
		return errors.New("stream canonical verdict source module is not supported")
	}
	if strings.TrimSpace(verdict.Target) != target || strings.TrimSpace(verdict.Network) != network {
		return errors.New("stream canonical verdict target/network mismatch")
	}
	if verdict.RulesetVersion != UnifiedRadarRulesetVersionV140 {
		return fmt.Errorf("stream canonical verdict requires %s", UnifiedRadarRulesetVersionV140)
	}
	if err := VerifyUnifiedRadarVerdictWithTrustedRegistry(verdict, registry); err != nil {
		return fmt.Errorf("stream canonical verdict authentication failed: %w", err)
	}
	for _, hit := range verdict.TriggeredRules {
		if strings.ToLower(strings.TrimSpace(hit.EvidenceStatus)) != "verified" {
			continue
		}
		for _, cited := range hit.Signatures {
			if strings.TrimSpace(cited) == signature {
				return nil
			}
		}
		for _, cited := range hit.EvidenceKeys {
			if strings.TrimSpace(cited) == "event-signature:"+signature {
				return nil
			}
		}
	}
	return errors.New("stream transaction is not cited by a verified rule in the authenticated canonical decision")
}
