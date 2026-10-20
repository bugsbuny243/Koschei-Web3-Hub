package handlers

import (
	"testing"
	"time"
)

func TestTransactionGuardStateRecheckPermitActiveAtFinalDecision(t *testing.T) {
	expiry := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC)
	claims := transactionGuardEnforcementPermitClaims{ExpiresAt: expiry.Format(time.RFC3339Nano)}

	if !transactionGuardStateRecheckPermitActiveAt(claims, expiry.Add(-time.Nanosecond)) {
		t.Fatal("the previously verified permit should be active immediately before its expiry")
	}
	if transactionGuardStateRecheckPermitActiveAt(claims, expiry) {
		t.Fatal("a permit is not active at its expiration instant")
	}
	if transactionGuardStateRecheckPermitActiveAt(claims, expiry.Add(time.Second)) {
		t.Fatal("slow evidence collection must not resurrect an expired permit")
	}
	if transactionGuardStateRecheckPermitActiveAt(claims, time.Time{}) {
		t.Fatal("missing decision time must fail closed")
	}
	claims.ExpiresAt = "malformed"
	if transactionGuardStateRecheckPermitActiveAt(claims, expiry.Add(-time.Second)) {
		t.Fatal("invalid permit expiry must fail closed")
	}
}
