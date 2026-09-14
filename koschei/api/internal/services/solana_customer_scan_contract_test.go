package services

import "testing"

func TestCustomerScanSolanaObservationRequiresTargetNetworkBinding(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "11111111111111111111111111111111",
		NetworkHint:    "ethereum-mainnet",
		Kind:           CustomerScanTargetSolana,
		Route:          CustomerScanRouteSolanaIntel,
		Classification: "syntax_only",
	}
	observation := SolanaAccountObservation{
		Address: target.Raw,
		Network: "solana-mainnet",
		Slot:    123,
		Present: false,
	}
	if _, err := CustomerScanResultFromSolanaObservation(target, observation); err == nil {
		t.Fatal("expected mismatched target network to reject Solana observation")
	}
}

func TestCustomerScanAbsentSolanaObservationStaysObservedNotSafe(t *testing.T) {
	target := CustomerScanTarget{
		Raw:            "11111111111111111111111111111111",
		NetworkHint:    "solana-mainnet",
		Kind:           CustomerScanTargetSolana,
		Route:          CustomerScanRouteSolanaIntel,
		Classification: "syntax_only",
	}
	observation := SolanaAccountObservation{
		Address: target.Raw,
		Network: target.NetworkHint,
		Slot:    123,
		Present: false,
	}
	result, err := CustomerScanResultFromSolanaObservation(target, observation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CustomerScanStatusObserved || result.Verdict != CustomerScanVerdictReview {
		t.Fatalf("absent account observation was misclassified: status=%q verdict=%q", result.Status, result.Verdict)
	}
	if result.Trust.Verified || result.Trust.Authorized || result.Trust.Finalized {
		t.Fatalf("absent Solana observation was over-promoted: %+v", result.Trust)
	}
}
