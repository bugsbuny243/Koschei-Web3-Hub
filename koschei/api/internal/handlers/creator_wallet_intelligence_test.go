package handlers

import "testing"

func TestCreatorIntelParsedTokenOwner(t *testing.T) {
	raw := map[string]any{
		"parsed": map[string]any{
			"info": map[string]any{"owner": "CreatorWallet111111111111111111111111111"},
		},
	}
	if got := creatorIntelParsedTokenOwner(raw); got != "CreatorWallet111111111111111111111111111" {
		t.Fatalf("unexpected owner: %q", got)
	}
	if got := creatorIntelParsedTokenOwner(map[string]any{}); got != "" {
		t.Fatalf("missing parsed owner must be empty, got %q", got)
	}
}

func TestCreatorIntelScoreEscalatesVerifiedBehavior(t *testing.T) {
	result := map[string]any{"previous_launch_count": 4}
	holders := creatorIntelHolderResult{CreatorIsTopHolder: true, CreatorRank: 1, CreatorPercentage: 58.4}
	score, level := creatorIntelScore(result, 2, 1, 3, 1, holders)
	if score < 75 || level != "critical" {
		t.Fatalf("expected critical creator behavior, got score=%d level=%s", score, level)
	}
}

func TestCreatorIntelFlowRowsMarksHolderLink(t *testing.T) {
	flows := map[string]*creatorIntelFlow{
		"WalletA": {Wallet: "WalletA", Amount: 10, Transactions: 2},
	}
	holders := map[string]map[string]any{
		"WalletA": {"rank": 3, "percentage": 12.5},
	}
	rows := creatorIntelFlowRows(flows, holders)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	matched, _ := rows[0]["matches_top_holder"].(bool)
	if !matched {
		t.Fatal("expected holder link")
	}
}
