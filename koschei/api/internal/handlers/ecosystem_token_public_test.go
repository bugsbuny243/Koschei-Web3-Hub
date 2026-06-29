package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicTokenStatusPlanningState(t *testing.T) {
	t.Setenv("KOSCHEI_TOKEN_MINT", "")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/token/status", nil)
	(&Handler{}).PublicTokenStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["phase"] != "planning" || body["configured"] != false {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestPublicTokenConcentration(t *testing.T) {
	accounts := []publicTokenLargestAccount{
		{Amount: "250"},
		{Amount: "150"},
		{Amount: "100"},
	}
	got := publicTokenConcentration(accounts, "1000", 2)
	if got < 39.999 || got > 40.001 {
		t.Fatalf("concentration = %f, want 40", got)
	}
}
