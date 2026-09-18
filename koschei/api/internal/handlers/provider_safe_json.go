package handlers

import (
	"bytes"
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

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "response_encode_failed"})
		return
	}

	payload, err = json.Marshal(providerSafeJSONValue(normalized))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "response_encode_failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func providerSafeJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return services.RedactProviderCredentials(typed)
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = providerSafeJSONValue(typed[index])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = providerSafeJSONValue(item)
		}
		return out
	default:
		return value
	}
}
