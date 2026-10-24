package http

import (
	"encoding/json"
	"net/http"

	"koschei/api/internal/services"
)

const verdictKeysPath = "/.well-known/koschei-verdict-keys.json"

// registerVerdictKeyRoutes publishes the public half of the verdict signer so
// anyone can verify signed ARVIS verdicts. Only public key material is served.
func registerVerdictKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc(verdictKeysPath, method("GET", publishedVerdictKeys))
}

func publishedVerdictKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	payload := map[string]any{
		"schema": "koschei.verdict-keys.v1",
		"keys":   []services.PublishedVerdictVerificationKey{},
	}
	status := http.StatusOK
	key, configured, err := services.ActiveVerdictVerificationKey()
	switch {
	case err != nil:
		payload["status"] = "signer_invalid"
		status = http.StatusServiceUnavailable
		w.Header().Set("Cache-Control", "no-store")
	case !configured:
		payload["status"] = "signer_unconfigured"
	default:
		payload["status"] = "active"
		payload["keys"] = []services.PublishedVerdictVerificationKey{key}
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
