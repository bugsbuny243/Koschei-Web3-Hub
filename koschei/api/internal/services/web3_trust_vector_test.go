package services

import "testing"

func TestValidateWeb3TrustVectorRejectsVerifiedWithoutObservation(t *testing.T) {
	if err := ValidateWeb3TrustVector(Web3TrustVector{Verified: true}); err == nil {
		t.Fatal("expected verified-without-observed trust vector to fail closed")
	}
}

func TestValidateWeb3TrustVectorRejectsFinalizedWithoutVerification(t *testing.T) {
	if err := ValidateWeb3TrustVector(Web3TrustVector{Observed: true, Finalized: true}); err == nil {
		t.Fatal("expected finalized-without-verified trust vector to fail closed")
	}
}

func TestValidateWeb3TrustVectorAllowsIndependentAuthorizationAndAvailability(t *testing.T) {
	vector := Web3TrustVector{Authorized: true, Available: true}
	if err := ValidateWeb3TrustVector(vector); err != nil {
		t.Fatalf("independent trust axes should remain valid: %v", err)
	}
	if got := Web3TrustEvidenceStatus(vector); got != Web3TrustEvidenceUnverified {
		t.Fatalf("authorization/availability must not promote evidence, got %q", got)
	}
}

func TestWeb3TrustEvidenceStatus(t *testing.T) {
	tests := []struct {
		name string
		in   Web3TrustVector
		want string
	}{
		{name: "unverified", in: Web3TrustVector{}, want: Web3TrustEvidenceUnverified},
		{name: "claimed", in: Web3TrustVector{Claimed: true}, want: Web3TrustEvidenceClaimed},
		{name: "observed", in: Web3TrustVector{Observed: true}, want: Web3TrustEvidenceObserved},
		{name: "verified", in: Web3TrustVector{Observed: true, Verified: true}, want: Web3TrustEvidenceVerified},
		{name: "finalized", in: Web3TrustVector{Observed: true, Verified: true, Finalized: true}, want: Web3TrustEvidenceFinalized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Web3TrustEvidenceStatus(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWeb3TrustReasons(t *testing.T) {
	got := NormalizeWeb3TrustReasons([]string{" ORACLE_STALE ", "", "ORACLE_STALE", "DA_EVIDENCE_INSUFFICIENT"})
	if len(got) != 2 || got[0] != "ORACLE_STALE" || got[1] != "DA_EVIDENCE_INSUFFICIENT" {
		t.Fatalf("unexpected normalized reasons: %#v", got)
	}
}
