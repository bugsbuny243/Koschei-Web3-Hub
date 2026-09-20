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

	verifiedStatus := SecurityRadarVerdict{
		Signals: map[string]any{"evidence_status": "verified_parsed_transaction"},
		Signed:  false,
	}
	if !SecurityRadarVerdictHasVerifiedEvidence(verifiedStatus) {
		t.Fatal("verified evidence status family must survive authentication migration")
	}

	observedStatus := SecurityRadarVerdict{
		Signals: map[string]any{"evidence_status": "observed_market_snapshot"},
		Signed:  true,
	}
	if SecurityRadarVerdictHasVerifiedEvidence(observedStatus) {
		t.Fatal("observed evidence must not become verified because it is authenticated")
	}

	authOnly := SecurityRadarVerdict{
		Signed:    true,
		Signature: "not-evidence",
	}
	if SecurityRadarVerdictHasVerifiedEvidence(authOnly) {
		t.Fatal("authentication alone must never be treated as verified evidence")
	}
}
