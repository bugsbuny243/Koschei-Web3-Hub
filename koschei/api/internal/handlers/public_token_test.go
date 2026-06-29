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
	if body["phase"] != "planning" {
		t.Fatalf("phase = %v, want planning", body["phase"])
	}
	if body["configured"] != false {
		t.Fatalf("configured = %v, want false", body["configured"])
	}
}

func TestPublicTokenMintAccountUnmarshal(t *testing.T) {
	payload := []byte(`{"value":{"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","executable":false,"data":{"parsed":{"type":"mint","info":{"decimals":6,"supply":"1000000","mintAuthority":null,"freezeAuthority":null}}}}}`)
	var account publicTokenMintAccount
	if err := json.Unmarshal(payload, &account); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if account.Owner != legacyTokenProgramID {
		t.Fatalf("owner = %q", account.Owner)
	}
	if account.Data.Parsed.Info.Decimals != 6 {
		t.Fatalf("decimals = %d", account.Data.Parsed.Info.Decimals)
	}
}

func TestTokenAccountConcentration(t *testing.T) {
	accounts := []struct {
		Address        string `json:"address"`
		Amount         string `json:"amount"`
		Decimals       int    `json:"decimals"`
		UIAmountString string `json:"uiAmountString"`
	}{
		{Amount: "250"},
		{Amount: "150"},
		{Amount: "100"},
	}
	got := tokenAccountConcentration(accounts, "1000", 2)
	if got < 39.999 || got > 40.001 {
		t.Fatalf("concentration = %f, want 40", got)
	}
}
