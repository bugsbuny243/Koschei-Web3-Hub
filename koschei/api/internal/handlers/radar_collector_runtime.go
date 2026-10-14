package handlers

import (
	"context"
	"encoding/json"
	"io"
	"koschei/api/internal/radaradapter"
	"koschei/api/internal/radaradapter/builtin"
	"koschei/api/internal/services"
	"log"
	"net/http"
	"time"
)

func (h *Handler) OwnerRadarCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "capabilities": services.RadarCapabilities(), "runtime": services.RadarCapabilityRuntime(), "catalog": builtin.Catalog()})
}
func (h *Handler) RadarInvestigate(w http.ResponseWriter, r *http.Request) {
	var target radaradapter.Target
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "invalid_target"})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || target.Raw == "" || len(target.Raw) > 512 || len(target.Network) > 64 || len(target.Kind) > 40 {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "invalid_target"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	result := services.ExecuteRadarInvestigation(ctx, target)
	status := http.StatusOK
	switch result.Status {
	case "declared_not_executable":
		status = 501
	case "invalid_target":
		status = 400
	case "insufficient_evidence":
		status = 503
	}
	log.Printf("radar collector network=%q kind=%q status=%s rows=%d requests=%d latency_ms=%d", target.Network, target.Kind, result.Status, result.EvidenceRows, result.Requests, result.LatencyMS)
	writeJSON(w, status, result)
}
