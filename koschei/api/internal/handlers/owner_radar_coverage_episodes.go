package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const ownerCoverageEpisodeDecisionBoundary = "coverage episodes are operational monitoring state; incident linkage records operator ownership without proving historical data loss, containment, or chain safety"

type ownerCoverageEpisodeIncident struct {
	ID          string `json:"id"`
	IncidentRef string `json:"incident_ref"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
}

type ownerCoverageEpisodeRecord struct {
	ID              string                         `json:"id"`
	ComponentID     string                         `json:"component_id"`
	NetworkID       string                         `json:"network_id"`
	Status          string                         `json:"status"`
	CoverageStatus  string                         `json:"coverage_status"`
	CoverageReason  string                         `json:"coverage_reason"`
	OpenAlertID     string                         `json:"open_alert_id,omitempty"`
	RecoveryAlertID string                         `json:"recovery_alert_id,omitempty"`
	OpenedAt        time.Time                      `json:"opened_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
	RecoveredAt     *time.Time                     `json:"recovered_at,omitempty"`
	Incidents       []ownerCoverageEpisodeIncident `json:"incidents"`
}

func (h *Handler) OwnerRadarCoverageEpisodes(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episode store is unavailable")
		return
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episode store is unavailable")
		return
	}

	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != "open" && status != "recovered" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid coverage episode status")
		return
	}
	network := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("network")))
	if len(network) > 96 {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid coverage episode network")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 250 {
		limit = 250
	}

	rows, err := db.QueryContext(r.Context(), `
		SELECT
			e.id::text,
			e.component_id,
			e.network_id,
			e.status,
			e.coverage_status,
			e.coverage_reason,
			COALESCE(e.open_alert_id::text,''),
			COALESCE(e.recovery_alert_id::text,''),
			e.opened_at,
			e.updated_at,
			e.recovered_at,
			COALESCE((
				SELECT jsonb_agg(
					jsonb_build_object(
						'id', c.id::text,
						'incident_ref', c.incident_ref,
						'status', c.status,
						'severity', c.severity
					)
					ORDER BY c.updated_at DESC, c.id DESC
				)
				FROM security_incident_cases c
				WHERE
					(e.open_alert_id IS NOT NULL AND c.alert_refs ? e.open_alert_id::text)
					OR
					(e.recovery_alert_id IS NOT NULL AND c.alert_refs ? e.recovery_alert_id::text)
			), '[]'::jsonb)
		FROM global_radar_coverage_episodes e
		WHERE ($1='' OR e.status=$1)
		  AND ($2='' OR e.network_id=$2)
		ORDER BY e.updated_at DESC, e.id DESC
		LIMIT $3
	`, status, network, limit)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episodes are unavailable")
		return
	}
	defer rows.Close()

	items := make([]ownerCoverageEpisodeRecord, 0)
	for rows.Next() {
		var item ownerCoverageEpisodeRecord
		var recovered sql.NullTime
		var incidentsRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.ComponentID,
			&item.NetworkID,
			&item.Status,
			&item.CoverageStatus,
			&item.CoverageReason,
			&item.OpenAlertID,
			&item.RecoveryAlertID,
			&item.OpenedAt,
			&item.UpdatedAt,
			&recovered,
			&incidentsRaw,
		); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episodes could not be decoded")
			return
		}
		if recovered.Valid {
			value := recovered.Time
			item.RecoveredAt = &value
		}
		item.Incidents = []ownerCoverageEpisodeIncident{}
		if err := json.Unmarshal(incidentsRaw, &item.Incidents); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episode ownership could not be decoded")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Coverage episodes are unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"coverage_episodes": items,
		"decision_boundary": ownerCoverageEpisodeDecisionBoundary,
	})
}
