package handlers

import (
	"net/http"
	"net/http/httptest"
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
