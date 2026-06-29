package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenLaunchReadinessFailsClosedWithoutConfiguration(t *testing.T) {
	for _, key := range []string{
		"KOSCHEI_TOKEN_LAUNCH_AT",
		"KOSCHEI_TOKEN_NAME",
		"KOSCHEI_TOKEN_SYMBOL",
		"KOSCHEI_TOKEN_MINT",
		"KOSCHEI_TOKEN_TREASURY",
		"KOSCHEI_TOKEN_DISCLOSURE_URL",
		"KOSCHEI_TOKEN_VESTING_URL",
		"KOSCHEI_TOKEN_BURN_ENABLED",
		"KOSCHEI_TOKEN_GATE_ENABLED",
	} {
		t.Setenv(key, "")
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/token/readiness", nil)
	(&Handler{}).PublicTokenLaunchReadiness(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		LaunchReady   bool `json:"launch_ready"`
		BlockingCount int  `json:"blocking_count"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.LaunchReady || body.BlockingCount == 0 {
		t.Fatalf("unexpected readiness: %#v", body)
	}
}

func TestTokenLaunchReadinessBlocksAutomaticBurn(t *testing.T) {
	t.Setenv("KOSCHEI_TOKEN_BURN_ENABLED", "true")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/token/readiness", nil)
	(&Handler{}).PublicTokenLaunchReadiness(recorder, request)
	var body struct {
		Checks []tokenLaunchCheck `json:"checks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, check := range body.Checks {
		if check.ID == "burn" {
			if !check.Blocking || check.Status != "blocked" {
				t.Fatalf("unexpected burn check: %#v", check)
			}
			return
		}
	}
	t.Fatal("burn check missing")
}
