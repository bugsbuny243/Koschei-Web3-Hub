package services

import "testing"

func TestSelectArvisTargetMintSkipsBaseAssets(t *testing.T) {
	projectMint := "DezXAZ8z7PnrnRJjz3wXBoRgixCa6kCmrWHp9xD9pump"
	mints := []string{
		"So11111111111111111111111111111111111111112",
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		projectMint,
	}
	if got := selectArvisTargetMint(mints); got != projectMint {
		t.Fatalf("expected project mint %s, got %s", projectMint, got)
	}
}

func TestSelectArvisTargetMintFallsBackToBaseAsset(t *testing.T) {
	wsol := "So11111111111111111111111111111111111111112"
	if got := selectArvisTargetMint([]string{wsol}); got != wsol {
		t.Fatalf("expected fallback mint %s, got %s", wsol, got)
	}
}

func TestSelectArvisTargetMintRejectsInvalidValues(t *testing.T) {
	if got := selectArvisTargetMint([]string{"", "not-a-mint"}); got != "" {
		t.Fatalf("expected empty selection, got %s", got)
	}
}
