package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFreeTokenScanIsNotBlockedByTierOrQuota(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	handler := NewServer(nil, "", "", "", "")
	req := httptest.NewRequest(http.MethodPost, "/api/token/scan", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code == http.StatusUnauthorized || recorder.Code == http.StatusForbidden || recorder.Code == http.StatusTooManyRequests {
		t.Fatalf("free core was gated by auth/tier/quota: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
