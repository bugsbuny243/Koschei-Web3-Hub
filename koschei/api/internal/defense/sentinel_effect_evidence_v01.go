package defense

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// EffectAgreement deliberately separates an executor acknowledgement from an
// independently observed effect. Unknown is fail-closed and is never success.
type EffectAgreement string

const (
	EffectAgree    EffectAgreement = "agree"
	EffectDisagree EffectAgreement = "disagree"
	EffectUnknown  EffectAgreement = "unknown"
)

// ObservedDefensiveEffect is evidence produced from an observation boundary
// distinct from the reasoning/execution acknowledgement path.
type ObservedDefensiveEffect struct {
	ActionID       string            `json:"action_id"`
	TargetID       string            `json:"target_id"`
	ObserverID     string            `json:"observer_id"`
	ExpectedState  string            `json:"expected_state"`
	ObservedState  string            `json:"observed_state"`
	EvidenceDigest string            `json:"evidence_digest"`
	ObservedAt     time.Time         `json:"observed_at"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

func (e ObservedDefensiveEffect) Agreement() EffectAgreement {
	if strings.TrimSpace(e.ExpectedState) == "" || strings.TrimSpace(e.ObservedState) == "" {
		return EffectUnknown
	}
	if strings.TrimSpace(e.ExpectedState) == strings.TrimSpace(e.ObservedState) {
		return EffectAgree
	}
	return EffectDisagree
}

func (e ObservedDefensiveEffect) Digest() string {
	encoded, err := json.Marshal(e)
	if err != nil {
		encoded = []byte("null")
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// RequireVerifiedDefensiveEffect is the fail-closed boundary used before an
// action may be recorded as successfully contained.
func RequireVerifiedDefensiveEffect(e ObservedDefensiveEffect) error {
	if strings.TrimSpace(e.ActionID) == "" || strings.TrimSpace(e.TargetID) == "" || strings.TrimSpace(e.ObserverID) == "" {
		return errors.New("observed effect identity is incomplete")
	}
	if strings.TrimSpace(e.EvidenceDigest) == "" {
		return errors.New("observed effect evidence digest is required")
	}
	switch e.Agreement() {
	case EffectAgree:
		return nil
	case EffectDisagree:
		return errors.New("observed defensive effect disagrees with expected state")
	default:
		return errors.New("observed defensive effect is unknown")
	}
}
