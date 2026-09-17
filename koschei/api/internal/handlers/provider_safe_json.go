package handlers

import (
	"encoding/json"
	"net/http"

	"koschei/api/internal/services"
)

func writeProviderSafeJSON(w http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "response_encode_failed"})
		return
	}
	payload = []byte(services.RedactProviderCredentials(string(payload)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
