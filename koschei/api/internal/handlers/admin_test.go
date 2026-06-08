package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminHandlersRejectMissingOrWrongPassword(t *testing.T) {
	h := &Handler{AdminPassword: "correct-password"}
	handlers := map[string]http.HandlerFunc{
		"summary":                 h.AdminSummary,
		"users":                   h.AdminUsers,
		"entitlements":            h.AdminEntitlements,
		"outputs":                 h.AdminOutputs,
		"watchlist-sources":       h.AdminWatchlistSources,
		"web3-events":             h.AdminWeb3Events,
		"chain-health":            h.AdminChainHealth,
		"analytics":               h.AdminAnalyticsEvents,
		"payment-requests":        h.AdminPaymentRequests,
		"payment-request-approve": h.ApprovePaymentRequest,
		"payment-request-reject":  h.RejectPaymentRequest,
		"chat":                    h.AdminChat,
	}

	for name, handler := range handlers {
		for _, password := range []string{"", "wrong-password"} {
			t.Run(name+"/"+password, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/admin/"+name, nil)
				if password != "" {
					req.Header.Set("x-admin-password", password)
				}
				res := httptest.NewRecorder()
				handler(res, req)
				if res.Code != http.StatusUnauthorized {
					t.Fatalf("expected HTTP 401, got %d", res.Code)
				}
			})
		}
	}
}

func TestAdminChatSupportsReadOnlyOperationalPrompts(t *testing.T) {
	summary := adminSummary{
		PendingPaymentRequestsCount: 2,
		WatchlistSourcesCount:       3,
		Web3EventsCount:             4,
		ChainHealthLogsCount:        5,
		AnalyticsEventsCount:        6,
		Web3OutputsCount:            7,
	}
	scan := adminScan{OK: true, Status: "healthy", Checks: []adminCheck{{Name: "env: ALCHEMY_API_KEY", Status: "ok", Message: "Environment setting is present."}}}
	tests := map[string]string{
		"Sistemi tara":            "System scan found no warnings",
		"Bugün neler olmuş?":      "Totals: 6 analytics events, 7 outputs, 4 web3 events",
		"Ödeme bekleyen var mı?":  "2 pending payment request(s)",
		"Wallet modülü aktif mi?": "six Pro production modules",
		"Alchemy çalışıyor mu?":   "5 chain health log(s)",
	}
	for prompt, want := range tests {
		t.Run(prompt, func(t *testing.T) {
			answer := adminChatAnswer(prompt, summary, nil, scan)
			if !strings.Contains(answer, want) {
				t.Fatalf("answer %q does not contain %q", answer, want)
			}
		})
	}
}
