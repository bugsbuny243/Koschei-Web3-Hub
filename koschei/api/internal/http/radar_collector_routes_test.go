package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRadarCollectorOwnerRoutesRequireIdentity(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "fixture-owner-radar")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("OWNER_WALLET", "")
	t.Setenv("RADAR_PROVIDERS_JSON", "{}")
	mux := NewServer(nil, "", "", "", t.TempDir())
	for _, path := range []string{"/api/owner/radar/capabilities", "/api/owner/radar/investigate"} {
		method := "GET"
		if strings.HasSuffix(path, "investigate") {
			method = "POST"
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
		if w.Code != http.StatusNotFound {
			t.Fatalf("anonymous owner route status=%d %s", w.Code, w.Body.String())
		}
	}
	req := httptest.NewRequest("GET", "/api/owner/radar/capabilities", nil)
	req.Header.Set("X-Owner-Secret", "fixture-owner-radar")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"live_verified":false`) {
		t.Fatalf("capability route %d %s", w.Code, w.Body.String())
	}
}
