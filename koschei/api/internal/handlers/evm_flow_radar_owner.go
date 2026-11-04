package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// OwnerEVMFlowRadar reports every in-memory EVM Flow Radar running in this
// process: per-network progress, followed addresses and recent findings. It
// reads process memory only; nothing here comes from or goes to the database.
func (h *Handler) OwnerEVMFlowRadar(w http.ResponseWriter, r *http.Request) {
	radars := services.ActiveEVMFlowRadars()
	if len(radars) == 0 {
		reason := "EVM Flow Radar is off. Set KOSCHEI_EVM_FLOW_RADAR_ENABLED=1 or true on a process that runs background workers; it follows USDT/USDC on the networks in KOSCHEI_EVM_FLOW_RADAR_NETWORKS (default ethereum-mainnet) using each network's existing *_RPC_URL."
		if services.EVMFlowRadarEnabled() {
			reason = "EVM Flow Radar is enabled but no network started in this process. Check that each network in KOSCHEI_EVM_FLOW_RADAR_NETWORKS has an https *_RPC_URL and that this process runs background workers."
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": false, "storage": "in_memory_only", "reason": reason})
		return
	}
	snapshots := make([]services.EVMFlowRadarSnapshot, 0, len(radars))
	for _, radar := range radars {
		snapshots = append(snapshots, radar.Snapshot())
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": true, "storage": "in_memory_only", "networks": snapshots})
}

// OwnerEVMFlowEvidencePackage returns a signed, downloadable evidence package
// for one in-memory EVM Flow Radar finding, identified by its dedupe key.
func (h *Handler) OwnerEVMFlowEvidencePackage(w http.ResponseWriter, r *http.Request) {
	if len(services.ActiveEVMFlowRadars()) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "evm_flow_radar_not_running"})
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("finding"))
	if key == "" || len(key) > 240 || !strings.HasPrefix(key, "evm-flow:") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "finding_dedupe_key_required"})
		return
	}
	finding, ok := services.FindEVMFlowFinding(key)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "finding_not_in_memory"})
		return
	}
	pkg, err := services.BuildEVMFlowEvidencePackage(finding, time.Now())
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
