package services

import (
	"errors"
	"strings"
)

const (
	Web3TrustEvidenceUnverified = "unverified"
	Web3TrustEvidenceClaimed    = "claimed"
	Web3TrustEvidenceObserved   = "observed"
	Web3TrustEvidenceVerified   = "verified"
	Web3TrustEvidenceFinalized  = "finalized"
)

// Web3TrustVector keeps orthogonal trust facts separate. Authorization and
// availability are intentionally not folded into evidence maturity because
// they may be established independently of observation/finality depending on
// the protocol (for example, pre-execution authorization or oracle data).
//
// The vector is descriptive evidence only. It must never be interpreted as a
// safety verdict by itself.
type Web3TrustVector struct {
	Claimed    bool     `json:"claimed"`
	Observed   bool     `json:"observed"`
	Authorized bool     `json:"authorized"`
	Available  bool     `json:"available"`
	Verified   bool     `json:"verified"`
	Finalized  bool     `json:"finalized"`
	Reasons    []string `json:"reasons,omitempty"`
}

// ValidateWeb3TrustVector rejects impossible evidence promotion. A verified
// fact must have been observed, and a finalized fact must first be verified.
// Authorization and availability remain independent axes and do not imply
// safety, verification, or finality.
func ValidateWeb3TrustVector(v Web3TrustVector) error {
	if v.Verified && !v.Observed {
		return errors.New("verified trust requires observed evidence")
	}
	if v.Finalized && !v.Verified {
		return errors.New("finalized trust requires verified evidence")
	}
	return nil
}

// Web3TrustEvidenceStatus returns the strongest evidence maturity represented
// by the vector. It deliberately ignores Authorized and Available because they
// are orthogonal trust facts rather than evidence maturity levels.
func Web3TrustEvidenceStatus(v Web3TrustVector) string {
	if v.Finalized && v.Verified && v.Observed {
		return Web3TrustEvidenceFinalized
	}
	if v.Verified && v.Observed {
		return Web3TrustEvidenceVerified
	}
	if v.Observed {
		return Web3TrustEvidenceObserved
	}
	if v.Claimed {
		return Web3TrustEvidenceClaimed
	}
	return Web3TrustEvidenceUnverified
}

// NormalizeWeb3TrustReasons trims, removes empty values and de-duplicates
// reason codes without changing their first-seen order.
func NormalizeWeb3TrustReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(reasons))
	normalized := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		if _, exists := seen[reason]; exists {
			continue
		}
		seen[reason] = struct{}{}
		normalized = append(normalized, reason)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
