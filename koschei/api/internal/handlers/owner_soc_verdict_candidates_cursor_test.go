package handlers

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestOwnerSOCVerdictCursorCanonicalRoundTrip(t *testing.T) {
	at := time.Date(2026, 11, 1, 1, 2, 3, 456789000, time.UTC)
	id := "11111111-2222-4333-8444-555555555555"
	token := ownerSOCVerdictCursor(at, id)
	gotTime, gotID, err := parseOwnerSOCVerdictCursor(token)
	if err != nil || !gotTime.Equal(at) || gotID != id {
		t.Fatalf("cursor did not round trip: time=%s id=%s err=%v", gotTime, gotID, err)
	}
	for _, bad := range []string{
		"", "a", token + "=", strings.Repeat("a", 161),
		base64.RawURLEncoding.EncodeToString([]byte("2026-11-01T01:02:03Z|not-a-uuid")),
		base64.RawURLEncoding.EncodeToString([]byte("2026-11-01T01:02:03+00:00|" + id)),
		base64.RawURLEncoding.EncodeToString([]byte("2026-11-01T01:02:03Z|" + id + "|extra")),
		base64.RawURLEncoding.EncodeToString([]byte("bad-date|" + id)),
	} {
		if _, _, err := parseOwnerSOCVerdictCursor(bad); err == nil {
			t.Fatalf("noncanonical historical cursor accepted: %q", bad)
		}
	}
}

func TestOwnerSOCVerdictPageFilterBounds(t *testing.T) {
	at := time.Date(2026, 11, 1, 1, 2, 3, 0, time.UTC)
	id := "11111111-2222-4333-8444-555555555555"
	cursor := ownerSOCVerdictCursor(at, id)
	network, target, boundary, uuid, err := ownerSOCVerdictPageFilter("network=solana-mainnet&target=MintA&cursor=" + cursor)
	if err != nil || network != "solana-mainnet" || target != "MintA" ||
		boundary == nil || uuid != id {
		t.Fatalf("valid owner history scope rejected: network=%s target=%s boundary=%v uuid=%v err=%v", network, target, boundary, uuid, err)
	}
	for _, bad := range []string{
		"network=solana-mainnet", "target=MintA",
		"target=MintA&network=solana-mainnet&cursor=bad",
		"network=solana-mainnet&target=MintA&target=MintB",
		"network=solana-mainnet&target=%0A",
		"network=solana-mainnet&target=MintA&cursor=",
		"network=solana-mainnet&target=MintA&limit=1000000",
		"network=solana-mainnet&target=%ZZ",
		"network=" + strings.Repeat("a", 65) + "&target=MintA",
	} {
		if _, _, _, _, err := ownerSOCVerdictPageFilter(bad); err == nil {
			t.Fatalf("invalid owner history filter accepted: %s", bad)
		}
	}
	if network, target, boundary, uuid, err := ownerSOCVerdictPageFilter(""); err != nil ||
		network != "" || target != "" || boundary != nil || uuid != nil {
		t.Fatalf("original unscoped candidate endpoint changed: %s %s %v %v %v", network, target, boundary, uuid, err)
	}
}
