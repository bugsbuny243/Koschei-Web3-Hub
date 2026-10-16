package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testRecipientAddress(mid string) string {
	return "12345678" + strings.Repeat(mid, 24) + "ABCDEFGH"
}

func TestRecipientShieldNoMatchDoesNotAuthorizeSending(t *testing.T) {
	wallet := testRecipientAddress("W")
	contacts := []string{testRecipientAddress("D"), testRecipientAddress("E")}
	evidence := map[string]any{
		"collected":             true,
		"signature_count":       2,
		"transaction_attempted": 2,
		"transaction_decoded":   2,
		"transaction_failed":    0,
		"coverage_status":       "bounded_recent_history",
	}
	// This candidate would have no collision if its visible anchors differ.
	candidate := "87654321" + strings.Repeat("C", 24) + "HGFEDCBA"
	out := evaluateAddressPoisoning(wallet, candidate, contacts, evidence)
	if out["policy"] == "allow" || out["policy"] != "warn" {
		t.Fatalf("bounded history cannot authenticate a recipient: %#v", out)
	}
	if out["finding_state"] != "none_observed_in_bounded_scope" ||
		out["enforcement_status"] != "not_connected" ||
		out["limitation"] != "UNKNOWN_OR_HISTORICAL_RECIPIENTS_NOT_RULED_OUT" {
		t.Fatalf("bounded no-match must retain evidence limits: %#v", out)
	}
	if out["coverage_status"] != "bounded_recent_history" {
		t.Fatalf("bounded coverage must not become full-history proof: %#v", out)
	}
}

func TestRecipientShieldMissingRPCNeverBecomesClearSend(t *testing.T) {
	out := evaluateAddressPoisoning(testRecipientAddress("W"), testRecipientAddress("C"),
		[]string{"87654321" + strings.Repeat("D", 24) + "HGFEDCBA"},
		map[string]any{"collected": false, "error": "provider_not_configured"})
	if out["policy"] == "allow" || out["coverage_status"] != "unavailable" {
		t.Fatalf("user-supplied contacts are not verified onchain proof: %#v", out)
	}
	if out["enforcement_status"] != "not_connected" {
		t.Fatal("read-only evaluator must not claim to block wallet transactions")
	}
}

func TestRecipientShieldLookalikeStillWarnsOrBlocks(t *testing.T) {
	candidate := testRecipientAddress("C")
	lookalike := testRecipientAddress("D")
	observed := evaluateAddressPoisoning(testRecipientAddress("W"), candidate, []string{lookalike},
		map[string]any{"collected": true, "coverage_status": "bounded_recent_history"})
	if observed["policy"] != "block" || observed["finding_state"] != "suspected_lookalike" {
		t.Fatalf("strong visual collision must remain visible: %#v", observed)
	}
	if observed["enforcement_status"] != "not_connected" {
		t.Fatalf("read-only recommendation must not impersonate wallet enforcement: %#v", observed)
	}
}

func TestRecipientShieldRejectsUnsupportedNetworksAndOversizedContacts(t *testing.T) {
	for _, body := range []string{
		`{"wallet":"some-wallet","candidate":"some-recipient","network":"ethereum-mainnet"}`,
		`{"wallet":"some-wallet","candidate":"some-recipient","network":"bitcoin-mainnet"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/address-poisoning/check", strings.NewReader(body))
		(&Handler{}).AddressPoisoningCheck(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "address_poisoning_unsupported_network") {
			t.Fatalf("cross-chain recipient must not be checked using Solana RPC: %d %s", rec.Code, rec.Body.String())
		}
	}
	contacts := make([]string, 41)
	for i := range contacts {
		contacts[i] = testRecipientAddress("T")
	}
	body := `{"wallet":"some-wallet","candidate":"some-recipient","network":"solana-mainnet","known_contacts":[` +
		`"` + strings.Join(contacts, `","`) + `"]}`
	rec := httptest.NewRecorder()
	(&Handler{}).AddressPoisoningCheck(rec, httptest.NewRequest(http.MethodPost, "/api/v1/address-poisoning/check", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too_many_known_contacts") {
		t.Fatalf("unbounded contact input must be rejected: %d %s", rec.Code, rec.Body.String())
	}
}
