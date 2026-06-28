package handlers

import (
	"net/http"
	"strings"
)

type aiMediaPromptRequest struct {
	Prompt string `json:"prompt"`
}

func (h *Handler) AIImageGenerate(w http.ResponseWriter, r *http.Request) {
	h.aiMediaNotImplemented(w, r, "image")
}

func (h *Handler) AIAudioGenerate(w http.ResponseWriter, r *http.Request) {
	h.aiMediaNotImplemented(w, r, "tts")
}

func (h *Handler) AIVideoCreate(w http.ResponseWriter, r *http.Request) {
	h.aiMediaNotImplemented(w, r, "video")
}

func (h *Handler) AIVideoStatus(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_id_required"})
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"job_id":          jobID,
		"status":          "not_implemented",
		"credits_charged": false,
	})
}

func (h *Handler) aiMediaNotImplemented(w http.ResponseWriter, r *http.Request, tool string) {
	if _, ok := userFromContext(r.Context()); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req aiMediaPromptRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error":           "tool_not_implemented_yet",
		"tool":            tool,
		"credits_charged": false,
	})
}
