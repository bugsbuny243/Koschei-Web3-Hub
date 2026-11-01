package defense

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestActiveResponseCurrentCheckpointRequiresTrustedDatabase(t *testing.T) {
	cp, _, keys, now := activeResponseSignedCheckpointFixture(t)
	result, err := VerifyCurrentShadowLedgerCheckpoint(context.Background(), nil, cp, keys, now)
	if err == nil || result.Status != "rejected" || result.ExecutionPerformed {
		t.Fatalf("missing database accepted as signed audit: %+v %v", result, err)
	}
}

func TestActiveResponseCurrentCheckpointRejectsNonGenesisOrUnknownSigner(t *testing.T) {
	cp, _, keys, now := activeResponseSignedCheckpointFixture(t)
	cp.StartHash = "sha256:" + strings.Repeat("1", 64)
	result, err := VerifyCurrentShadowLedgerCheckpoint(context.Background(), nil, cp, keys, now)
	if err == nil || result.Status != "rejected" {
		t.Fatalf("non-genesis checkpoint accepted: %+v %v", result, err)
	}
	cp, _, keys, now = activeResponseSignedCheckpointFixture(t)
	result, err = VerifyCurrentShadowLedgerCheckpoint(context.Background(), nil, cp, nil, now)
	if err == nil || result.Status != "rejected" {
		t.Fatalf("untrusted signer accepted: %+v %v", result, err)
	}
}

func TestActiveResponseCurrentCheckpointRejectsCanceledOrFutureVerification(t *testing.T) {
	cp, _, keys, now := activeResponseSignedCheckpointFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := VerifyCurrentShadowLedgerCheckpoint(ctx, nil, cp, keys, now)
	if err != context.Canceled || result.Status != "rejected" {
		t.Fatalf("canceled verification accepted: %+v %v", result, err)
	}
	result, err = VerifyCurrentShadowLedgerCheckpoint(context.Background(), nil,
		cp, keys, cp.IssuedAt.Add(-time.Second))
	if err == nil || result.Status != "rejected" {
		t.Fatalf("future signed checkpoint accepted: %+v %v", result, err)
	}
}
