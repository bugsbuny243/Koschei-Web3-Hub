package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicToolPricesUsesSingleProfessionalContract(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/tool-prices", nil)

	(&Handler{}).PublicToolPrices(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Plan string `json:"plan"`
		PriceUSD int `json:"price_usd"`
		AccessDays int `json:"access_days"`
		PaymentPaths []map[string]any `json:"payment_paths"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != "koschei-professional-access-v1" || payload.Plan != "professional" || payload.PriceUSD != 199 || payload.AccessDays != 30 {
		t.Fatalf("unexpected Professional contract: %+v", payload)
	}
	if len(payload.PaymentPaths) != 2 || payload.PaymentPaths[0]["provider"] != "polar" || payload.PaymentPaths[1]["provider"] != "kosc" {
		t.Fatalf("unexpected payment paths: %#v", payload.PaymentPaths)
	}
}
