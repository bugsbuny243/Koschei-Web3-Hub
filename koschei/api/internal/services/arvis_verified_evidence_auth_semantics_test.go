package services

import "testing"

func TestSecurityRadarVerifiedEvidenceIsIndependentFromAuthentication(t *testing.T) {
	unsigned := SecurityRadarVerdict{
		EvidenceVerified: true,
		Signed:           false,
		Signature:        "",
	}
	if !SecurityRadarVerdictHasVerifiedEvidence(unsigned) {
		t.Fatal("verified evidence must not depend on cryptographic authentication")
	}

	legacySignal := SecurityRadarVerdict{
		Signals: map[string]any{"real_onchain_evidence": true},
		Signed:  false,
	}
	if !SecurityRadarVerdictHasVerifiedEvidence(legacySignal) {
		t.Fatal("legacy evidence signals must remain readable during authentication migration")
	}

	authOnly := SecurityRadarVerdict{
		Signed:    true,
		Signature: "not-evidence",
	}
	if SecurityRadarVerdictHasVerifiedEvidence(authOnly) {
		t.Fatal("authentication alone must never be treated as verified evidence")
	}
}
