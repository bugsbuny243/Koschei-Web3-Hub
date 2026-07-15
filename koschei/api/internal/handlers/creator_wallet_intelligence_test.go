package handlers

import (
	"strings"
	"testing"
)

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

func TestCreatorIntelSummaryIsScorelessEvidenceLayer(t *testing.T) {
	result := map[string]any{
		"creator_wallet":               "CreatorWallet111111111111111111111111111",
		"previous_launch_count":        4,
		"early_sale_like_transactions": 1,
		"creator_is_top_holder":        true,
		"creator_holder_rank":          1,
		"creator_holder_percentage":    58.4,
		"holder_links":                 []map[string]any{{"wallet": "HolderWallet111"}},
	}
	summary := creatorIntelSummary(result)
	if strings.Contains(summary, "/100") || strings.Contains(summary, "CRITICAL") || strings.Contains(summary, "HIGH") {
		t.Fatalf("creator intelligence summary must remain scoreless, got %q", summary)
	}
	if !strings.Contains(summary, "unified Radar ruleset v1.0") {
		t.Fatalf("summary must delegate risk impact to unified ruleset, got %q", summary)
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
