package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTronWalletPermissionsRejectsInvalidAddressWithoutNetworkIO(t *testing.T) {
	h := &Handler{}
	for _, query := range []string{"", "?address=not-a-tron-address", "?address=" + strings.Repeat("a", 129)} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/radar/tron-wallet-permissions"+query, nil)
		response := httptest.NewRecorder()
		h.TronWalletPermissions(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid address %q: status=%d want=400", query, response.Code)
		}
	}
}
