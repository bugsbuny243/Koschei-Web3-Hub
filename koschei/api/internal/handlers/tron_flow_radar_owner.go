package handlers

import (
	"context"
	"database/sql"
	"errors"
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
	key := strings.TrimSpace(r.URL.Query().Get("finding"))
	if key == "" || len(key) > 200 || !strings.HasPrefix(key, "tron-flow:") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "finding_dedupe_key_required"})
		return
	}
	var finding services.TronFlowFinding
	var ok bool
	if radar := services.ActiveTronFlowRadar(); radar != nil {
		finding, ok = radar.FindingByDedupeKey(key)
	}
	if !ok {
		var err error
		finding, err = h.persistedTronFlowFinding(r.Context(), key)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "finding_not_found"})
			case errors.Is(err, errTronFlowNoStore):
				writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "tron_flow_radar_not_running"})
			default:
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "evidence_store_unavailable"})
			}
			return
		}
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

var errTronFlowNoStore = errors.New("tron flow evidence store not configured")

// persistedTronFlowFinding retrieves a durable alert after a worker restart or
// in a split API/worker deployment. Malformed records must never be signed.
func (h *Handler) persistedTronFlowFinding(ctx context.Context, key string) (services.TronFlowFinding, error) {
	if h == nil {
		return services.TronFlowFinding{}, errTronFlowNoStore
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		return services.TronFlowFinding{}, errTronFlowNoStore
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var finding services.TronFlowFinding
	var payload []byte
	err := db.QueryRowContext(ctx, `SELECT target, severity, title, message, dedupe_key, payload
		FROM security_alert_events WHERE source = 'tron_flow_radar' AND dedupe_key = $1
		ORDER BY last_seen_at DESC LIMIT 1`, key).Scan(
		&finding.Address, &finding.Severity, &finding.Title, &finding.Message, &finding.DedupeKey, &payload)
	if err != nil {
		return services.TronFlowFinding{}, err
	}
	var detail struct {
		Kind string `json:"kind"`
		ObservedAt time.Time `json:"observed_at"`
		Evidence map[string]any `json:"evidence"`
	}
	if len(payload) > 1<<20 || json.Unmarshal(payload, &detail) != nil ||
		detail.Kind == "" || detail.ObservedAt.IsZero() || detail.Evidence == nil ||
		finding.DedupeKey != key {
		return services.TronFlowFinding{}, errors.New("invalid persisted tron flow evidence")
	}
	finding.Kind, finding.ObservedAt, finding.Evidence = detail.Kind, detail.ObservedAt, detail.Evidence
	return finding, nil
}
