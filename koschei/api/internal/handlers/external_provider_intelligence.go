package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"koschei/api/internal/providerintel"
)

// ExternalProviderIntelligence is an API-key protected read-only intelligence
// proxy. The result is NOT a source-verified or signed ARVIS risk verdict.
func (h *Handler) ExternalProviderIntelligence(w http.ResponseWriter, r *http.Request) {
	var input providerintel.Input
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_provider_request"})
		return
	}
	if len(input.Payload) > 32768 || len(input.Addresses) > 20 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "provider_request_too_large"})
		return
	}
	result, err := providerintel.NewFromEnv().Query(r.Context(), input)
	if err != nil {
		status := http.StatusBadGateway
		code := "provider_error"
		if errors.Is(err, providerintel.ErrInvalid) {
			status = http.StatusBadRequest
			code = "invalid_provider_request"
		} else if errors.Is(err, providerintel.ErrUnavailable) && strings.Contains(err.Error(), "credential not configured") {
			status = http.StatusServiceUnavailable
			code = "provider_not_configured"
		}
		writeJSON(w, status, map[string]any{
			"ok":         false,
			"error":      code,
			"risk_level": "unknown",
			"signed":     false,
			"provider":   input.Provider,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"source":       result.Source,
		"provider":     result.Provider,
		"operation":    result.Operation,
		"verification": result.Verification,
		"risk_level":   "unknown",
		"signed":       false,
		"data":         result.Data,
	})
}

// ExternalProviderCapabilities reports operator configuration, never a claim
// that a live provider or its quota is available.
func (h *Handler) ExternalProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                    true,
		"blockaid_configured":   strings.TrimSpace(os.Getenv("KOSCHEI_BLOCKAID_API_KEY")) != "",
		"goplus_configured":     strings.TrimSpace(os.Getenv("KOSCHEI_GOPLUS_ACCESS_TOKEN")) != "",
		"bubblemaps_configured": strings.TrimSpace(os.Getenv("KOSCHEI_BUBBLEMAPS_API_KEY")) != "",
		"slither":               "isolated_json_import_only",
		"result_authority":      "external_observation_only",
	})
}
