package handlers

import (
	"context"
	"testing"
)

func TestOwnerSOCSignedReceiptReadinessAbsentStorageFailsClosed(t *testing.T) {
	got := probeOwnerSOCSignedReceipts(context.Background(), nil)
	if got.State != "unavailable_not_accepted" || got.LedgerTableReadable ||
		got.IndependentTrustLoaded || got.SampledReceipts != 0 || got.VerifiedReceipts != 0 {
		t.Fatalf("unavailable ledger was labelled healthy: %+v", got)
	}
	if got.Scope != ownerSOCReceiptProbeScope || got.Boundary != ownerSOCReceiptProbeBoundary {
		t.Fatalf("bounded proof or live-acceptance limit was not disclosed: %+v", got)
	}
}
