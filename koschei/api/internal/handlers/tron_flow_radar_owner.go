package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// OwnerTronFlowRadar reports the in-memory Tron Flow Radar: progress, the
// addresses currently followed from seed wallets and recent findings. It reads
// process memory only; nothing here comes from or goes to the database.
func (h *Handler) OwnerTronFlowRadar(w http.ResponseWriter, r *http.Request) {
	radar := services.ActiveTronFlowRadar()
	if radar == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "enabled": false, "storage": "in_memory_only",
			"reason": "Tron Flow Radar is off. Set KOSCHEI_TRON_FLOW_RADAR_ENABLED=1 or true (TRONGRID_API_KEY required) on a process that runs background workers.",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": true, "radar": radar.Snapshot()})
}

var tronFlowEvidenceFilePart = regexp.MustCompile(`[^a-z0-9-]+`)

// OwnerTronFlowEvidencePackage returns a signed, downloadable evidence package
// for one in-memory Tron Flow Radar finding, identified by its dedupe key, to
// send to the party that can hold the funds.
func (h *Handler) OwnerTronFlowEvidencePackage(w http.ResponseWriter, r *http.Request) {
	radar := services.ActiveTronFlowRadar()
	if radar == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "tron_flow_radar_not_running"})
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("finding"))
	if key == "" || len(key) > 200 || !strings.HasPrefix(key, "tron-flow:") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "finding_dedupe_key_required"})
		return
	}
	finding, ok := radar.FindingByDedupeKey(key)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "finding_not_in_memory"})
		return
	}
	pkg, err := services.BuildTronFlowEvidencePackage(finding, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "evidence_package_unavailable"})
		return
	}
	name := tronFlowEvidenceFilePart.ReplaceAllString(strings.ToLower(strings.ReplaceAll(key, ":", "-")), "")
	if len(name) > 90 {
		name = name[:90]
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="koschei-`+name+`.json"`)
	w.Header().Set("Cache-Control", "no-store")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(pkg)
}
