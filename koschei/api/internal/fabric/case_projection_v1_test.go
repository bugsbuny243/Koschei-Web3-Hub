package fabric

import (
	"strings"
	"testing"
	"time"
)

func baseProjectionInput() Web3ProjectionInputV1 {
	return Web3ProjectionInputV1{
		CaseID:                      "case:fixture:1",
		CreatedAt:                   time.Date(2026, 9, 9, 4, 0, 0, 0, time.UTC),
		RequestDigestSHA256:         strings.Repeat("a", 64),
		Network:                     "solana-mainnet",
		Target:                      "fixture-target",
		RequestedOperation:          "report:read",
		RequestedEffect:             "read security report",
		ActualEffectState:           "NONE",
		IndependentObservationState: "UNVERIFIED",
		NativeSchema:                "arvis.export.v1",
		NativeRef:                   "fixture://arvis/export/1",
		NativeDigestSHA256:          strings.Repeat("b", 64),
		MappingState:                "PARTIAL",
	}
}

func TestBuildWeb3ProjectionV1PreservesOwnership(t *testing.T) {
	projection, err := BuildWeb3ProjectionV1(baseProjectionInput())
	if err != nil {
		t.Fatalf("BuildWeb3ProjectionV1() error = %v", err)
	}
	if projection.SchemaVersion != SecurityCaseEnvelopeV1 {
		t.Fatalf("schema version = %q", projection.SchemaVersion)
	}
	if projection.Case.Coordinator != "koschei-web3" {
		t.Fatalf("coordinator = %q", projection.Case.Coordinator)
	}
	if projection.NativeBinding.Owner != "koschei-web3" {
		t.Fatalf("native owner = %q", projection.NativeBinding.Owner)
	}
	if projection.Effect.ActualEffectState != "NONE" {
		t.Fatalf("actual effect state = %q", projection.Effect.ActualEffectState)
	}
}

func TestBuildWeb3ProjectionV1RejectsChangedOrMalformedDigest(t *testing.T) {
	input := baseProjectionInput()
	input.RequestDigestSHA256 = "not-a-digest"
	if _, err := BuildWeb3ProjectionV1(input); err == nil {
		t.Fatal("expected malformed request digest to fail")
	}
}

func TestBuildWeb3ProjectionV1DoesNotTreatUnverifiedObservationAsVerified(t *testing.T) {
	projection, err := BuildWeb3ProjectionV1(baseProjectionInput())
	if err != nil {
		t.Fatalf("BuildWeb3ProjectionV1() error = %v", err)
	}
	if projection.Effect.IndependentObservationState != "UNVERIFIED" {
		t.Fatalf("observation state = %q, want UNVERIFIED", projection.Effect.IndependentObservationState)
	}
}

func TestBuildWeb3ProjectionV1RequiresReceiptForVerifiedObservation(t *testing.T) {
	input := baseProjectionInput()
	input.IndependentObservationState = "VERIFIED"
	if _, err := BuildWeb3ProjectionV1(input); err == nil {
		t.Fatal("expected VERIFIED observation without receipt digest to fail")
	}

	receipt := strings.Repeat("c", 64)
	input.ReceiptDigestSHA256 = &receipt
	if _, err := BuildWeb3ProjectionV1(input); err != nil {
		t.Fatalf("verified observation with receipt digest failed: %v", err)
	}
}

func TestBuildWeb3ProjectionV1RejectsUnsupportedEffectState(t *testing.T) {
	input := baseProjectionInput()
	input.ActualEffectState = "SAFE"
	if _, err := BuildWeb3ProjectionV1(input); err == nil {
		t.Fatal("expected unsupported effect state to fail")
	}
}
