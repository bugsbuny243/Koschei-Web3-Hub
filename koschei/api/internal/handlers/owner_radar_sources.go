package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"koschei/api/internal/services"
)

type ownerRadarSourceRequest struct {
	ID       string `json:"id"`
	ModuleID string `json:"module_id"`
	Label    string `json:"label"`
	Address  string `json:"address"`
	Network  string `json:"network"`
	Enabled  *bool  `json:"enabled"`
}

func (h *Handler) OwnerRadarSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ownerListRadarSources(w, r)
	case http.MethodPost:
		h.OwnerCreateRadarSource(w, r)
	case http.MethodPatch:
		h.OwnerUpdateRadarSource(w, r)
	case http.MethodDelete:
		h.OwnerDeleteRadarSource(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method_not_allowed"})
	}
}

func (h *Handler) ownerListRadarSources(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sources": []services.SecurityRadarSource{}})
		return
	}
	items, err := services.NewSecurityRadarStore(h.DBRead).ListSources(r.Context())
	if err != nil {
		writeRadarSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sources": items, "message": "No fake sources are seeded. Worker waits until verified source/program addresses are added here."})
}

func (h *Handler) OwnerCreateRadarSource(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "database unavailable"})
		return
	}
	var req ownerRadarSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_body"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created, err := services.NewSecurityRadarStore(h.DB).CreateSource(r.Context(), services.SecurityRadarSource{ModuleID: req.ModuleID, Label: req.Label, Address: req.Address, Network: req.Network, Enabled: enabled})
	if err != nil {
		writeRadarSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": created})
}

func (h *Handler) OwnerUpdateRadarSource(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "database unavailable"})
		return
	}
	var req ownerRadarSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_body"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	updated, err := services.NewSecurityRadarStore(h.DB).UpdateSource(r.Context(), req.ID, services.SecurityRadarSource{ModuleID: req.ModuleID, Label: req.Label, Address: req.Address, Network: req.Network, Enabled: enabled})
	if err != nil {
		writeRadarSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": updated})
}

func (h *Handler) OwnerDisableRadarSource(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "database unavailable"})
		return
	}
	var req ownerRadarSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_body"})
		return
	}
	enabled := false
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := services.NewSecurityRadarStore(h.DB).SetSourceEnabled(r.Context(), req.ID, enabled); err != nil {
		writeRadarSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": strings.TrimSpace(req.ID), "enabled": enabled})
}

func (h *Handler) OwnerDeleteRadarSource(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "database unavailable"})
		return
	}
	var req ownerRadarSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_body"})
		return
	}
	if err := services.NewSecurityRadarStore(h.DB).DeleteSource(r.Context(), req.ID); err != nil {
		writeRadarSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": strings.TrimSpace(req.ID), "deleted": true})
}

func writeRadarSourceError(w http.ResponseWriter, err error) {
	msg := strings.TrimSpace(err.Error())
	switch {
	case errors.Is(err, services.ErrInvalidRadarSource):
		if strings.Contains(msg, "invalid Solana source address") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid Solana source address"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": msg})
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "radar source not found"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "radar source registry unavailable"})
	}
}
