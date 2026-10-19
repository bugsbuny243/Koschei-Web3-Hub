package services

import (
	"context"
	"errors"
	"testing"
)

// The startup guard is not enough: package-internal callers and future
// recovery paths must never bypass the canonical final persistence gate.
func TestArvisStreamWorkerDirectPathsFailClosedBeforeClaimOrCollection(t *testing.T) {
	if arvisStreamCanonicalFinalPipelineWired {
		t.Skip("canonical stream final pipeline is now wired: update integration acceptance")
	}

	target := arvisStreamTarget{
		ID: "00000000-0000-4000-8000-000000000001",
		Target: "So11111111111111111111111111111111111111112",
		Signature: "untrusted-event-reference",
		Network: "solana-mainnet",
		ModuleID: ModulePumpSybilRadar,
	}
	// No DB or collector is configured. Any attempt to claim the event through
	// processOne would panic here; a direct call must remain a no-op.
	worker := &arvisStreamVerdictWorker{}
	worker.processOne(context.Background(), target)

	// Processing a target directly cannot initiate a live collector scan when
	// signed final persistence is disconnected. The operational failure must
	// not be misclassified as a genuine lack of evidence.
	err := worker.processTarget(context.Background(), target)
	if !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
		t.Fatalf("direct processTarget: got %v, want canonical final unavailable", err)
	}
	if errors.Is(err, errArvisStreamInsufficientEvidence) {
		t.Fatal("missing canonical integration was mislabeled as insufficient evidence")
	}
}
