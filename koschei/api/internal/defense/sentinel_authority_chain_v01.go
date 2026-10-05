package defense

import (
	"errors"
	"fmt"
	"strings"
)

// SentinelAuthorityGrant is a bounded defensive capability delegated to the
// next actor in a Sentinel action chain. It never grants arbitrary execution.
type SentinelAuthorityGrant struct {
	GrantID      string   `json:"grant_id"`
	IssuerID     string   `json:"issuer_id"`
	SubjectID    string   `json:"subject_id"`
	TenantID     string   `json:"tenant_id"`
	Capabilities []string `json:"capabilities"`
	Targets      []string `json:"targets"`
	EvidenceIDs  []string `json:"evidence_ids"`
}

// VerifySentinelAuthorityChain requires every downstream hop to be an
// attenuation of the authority immediately above it. Empty chains and
// authority expansion fail closed.
func VerifySentinelAuthorityChain(chain []SentinelAuthorityGrant) error {
	if len(chain) == 0 {
		return errors.New("authority chain is required")
	}
	seen := map[string]struct{}{}
	for i, grant := range chain {
		if err := validateSentinelGrant(grant); err != nil {
			return fmt.Errorf("authority grant %d: %w", i, err)
		}
		if _, ok := seen[grant.GrantID]; ok {
			return fmt.Errorf("authority grant %d: duplicate grant id", i)
		}
		seen[grant.GrantID] = struct{}{}
		if i == 0 {
			continue
		}
		parent := chain[i-1]
		if grant.IssuerID != parent.SubjectID {
			return fmt.Errorf("authority grant %d: issuer does not match parent subject", i)
		}
		if grant.TenantID != parent.TenantID {
			return fmt.Errorf("authority grant %d: tenant boundary changed", i)
		}
		if !stringSubset(grant.Capabilities, parent.Capabilities) {
			return fmt.Errorf("authority grant %d: capability authority expanded", i)
		}
		if !stringSubset(grant.Targets, parent.Targets) {
			return fmt.Errorf("authority grant %d: target authority expanded", i)
		}
		if !stringSubset(grant.EvidenceIDs, parent.EvidenceIDs) {
			return fmt.Errorf("authority grant %d: evidence scope expanded", i)
		}
	}
	return nil
}

func validateSentinelGrant(grant SentinelAuthorityGrant) error {
	if strings.TrimSpace(grant.GrantID) == "" || strings.TrimSpace(grant.IssuerID) == "" || strings.TrimSpace(grant.SubjectID) == "" || strings.TrimSpace(grant.TenantID) == "" {
		return errors.New("grant identity is incomplete")
	}
	if len(grant.Capabilities) == 0 || len(grant.Targets) == 0 || len(grant.EvidenceIDs) == 0 {
		return errors.New("capability, target and evidence scopes are required")
	}
	return nil
}

func stringSubset(child, parent []string) bool {
	allowed := make(map[string]struct{}, len(parent))
	for _, value := range parent {
		value = strings.TrimSpace(value)
		if value != "" {
			allowed[value] = struct{}{}
		}
	}
	for _, value := range child {
		value = strings.TrimSpace(value)
		if value == "" {
			return false
		}
		if _, ok := allowed[value]; !ok {
			return false
		}
	}
	return true
}
