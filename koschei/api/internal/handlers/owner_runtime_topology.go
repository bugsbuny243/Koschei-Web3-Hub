package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"koschei/api/internal/runtimeinstance"
)

const ownerRuntimeTopologySchemaVersion = "koschei.runtime-topology.v1"

type ownerRuntimeInstance struct {
	InstanceID               string     `json:"instance_id"`
	RuntimeRole              string     `json:"runtime_role"`
	ServiceName              string     `json:"service_name,omitempty"`
	EnvironmentName          string     `json:"environment_name,omitempty"`
	DeploymentID             string     `json:"deployment_id,omitempty"`
	ReplicaID                string     `json:"replica_id,omitempty"`
	Region                   string     `json:"region,omitempty"`
	HTTPEnabled              bool       `json:"http_enabled"`
	BackgroundWorkersEnabled bool       `json:"background_workers_enabled"`
	State                    string     `json:"state"`
	StartedAt                time.Time  `json:"started_at"`
	HeartbeatAt              time.Time  `json:"heartbeat_at"`
	StoppedAt                *time.Time `json:"stopped_at,omitempty"`
	HeartbeatAgeSeconds      int64      `json:"heartbeat_age_seconds"`
}

type ownerRuntimeTopologyCounts struct {
	Active              int `json:"active"`
	Stale               int `json:"stale"`
	Stopped             int `json:"stopped"`
	ActiveHTTP          int `json:"active_http_capable"`
	ActiveBackground    int `json:"active_background_capable"`
	ActiveAPIOnly       int `json:"active_api_only"`
	ActiveWorkerOnly    int `json:"active_worker_only"`
	ActiveCombined      int `json:"active_combined"`
	DistinctServices    int `json:"distinct_active_services"`
	DistinctRegions     int `json:"distinct_active_regions"`
}

type ownerRuntimeTopologyTruthBoundary struct {
	SharedDatabaseHeartbeat bool   `json:"shared_database_heartbeat"`
	TrafficRoutingProof     bool   `json:"traffic_routing_proof"`
	WorkerCorrectnessProof  bool   `json:"worker_correctness_proof"`
	ChainCoverageProof      bool   `json:"chain_coverage_proof"`
	StaleAfterSeconds       int64  `json:"stale_after_seconds"`
	HistoryWindow           string `json:"history_window"`
}

type ownerRuntimeTopologyResponse struct {
	SchemaVersion       string                            `json:"schema_version"`
	GeneratedAt         time.Time                         `json:"generated_at"`
	Status              string                            `json:"status"`
	SplitDeploymentSeen bool                              `json:"split_deployment_observed"`
	Counts              ownerRuntimeTopologyCounts        `json:"counts"`
	Instances           []ownerRuntimeInstance            `json:"instances"`
	TruthBoundary       ownerRuntimeTopologyTruthBoundary `json:"truth_boundary"`
}

func (h *Handler) OwnerRuntimeTopology(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Runtime topology store is unavailable")
		return
	}
	now := time.Now().UTC()
	rows, err := db.QueryContext(r.Context(), `
		SELECT instance_id,runtime_role,service_name,environment_name,deployment_id,replica_id,region,
		       http_enabled,background_workers_enabled,started_at,heartbeat_at,stopped_at
		FROM runtime_instance_heartbeats
		WHERE heartbeat_at > now() - interval '7 days'
		   OR stopped_at > now() - interval '7 days'
		ORDER BY heartbeat_at DESC, instance_id ASC
		LIMIT 500
	`)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Runtime topology could not be measured")
		return
	}
	defer rows.Close()

	instances := make([]ownerRuntimeInstance, 0)
	counts := ownerRuntimeTopologyCounts{}
	services := map[string]struct{}{}
	regions := map[string]struct{}{}
	for rows.Next() {
		var item ownerRuntimeInstance
		var stopped sql.NullTime
		if err := rows.Scan(
			&item.InstanceID, &item.RuntimeRole, &item.ServiceName, &item.EnvironmentName,
			&item.DeploymentID, &item.ReplicaID, &item.Region,
			&item.HTTPEnabled, &item.BackgroundWorkersEnabled,
			&item.StartedAt, &item.HeartbeatAt, &stopped,
		); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Runtime topology row could not be decoded")
			return
		}
		if stopped.Valid {
			value := stopped.Time.UTC()
			item.StoppedAt = &value
		}
		item.State, item.HeartbeatAgeSeconds = classifyRuntimeInstance(now, item.HeartbeatAt, item.StoppedAt)
		switch item.State {
		case "active":
			counts.Active++
			if item.HTTPEnabled {
				counts.ActiveHTTP++
			}
			if item.BackgroundWorkersEnabled {
				counts.ActiveBackground++
			}
			switch item.RuntimeRole {
			case "api":
				counts.ActiveAPIOnly++
			case "worker":
				counts.ActiveWorkerOnly++
			case "combined":
				counts.ActiveCombined++
			}
			if item.ServiceName != "" {
				services[item.ServiceName] = struct{}{}
			}
			if item.Region != "" {
				regions[item.Region] = struct{}{}
			}
		case "stale":
			counts.Stale++
		case "stopped":
			counts.Stopped++
		}
		instances = append(instances, item)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Runtime topology query was interrupted")
		return
	}
	counts.DistinctServices = len(services)
	counts.DistinctRegions = len(regions)

	status := "observed"
	switch {
	case counts.Active == 0:
		status = "no_active_instances"
	case counts.ActiveHTTP == 0:
		status = "http_capability_missing"
	case counts.ActiveBackground == 0:
		status = "background_worker_capability_missing"
	}

	writeJSON(w, http.StatusOK, ownerRuntimeTopologyResponse{
		SchemaVersion:       ownerRuntimeTopologySchemaVersion,
		GeneratedAt:         now,
		Status:              status,
		SplitDeploymentSeen: counts.ActiveAPIOnly > 0 && counts.ActiveWorkerOnly > 0,
		Counts:              counts,
		Instances:           instances,
		TruthBoundary: ownerRuntimeTopologyTruthBoundary{
			SharedDatabaseHeartbeat: true,
			TrafficRoutingProof:     false,
			WorkerCorrectnessProof:  false,
			ChainCoverageProof:      false,
			StaleAfterSeconds:       int64(runtimeinstance.StaleAfter / time.Second),
			HistoryWindow:           "7_days_bounded_500_rows",
		},
	})
}

func classifyRuntimeInstance(now, heartbeatAt time.Time, stoppedAt *time.Time) (string, int64) {
	age := now.UTC().Sub(heartbeatAt.UTC())
	if age < 0 {
		age = 0
	}
	ageSeconds := int64(age / time.Second)
	if stoppedAt != nil {
		return "stopped", ageSeconds
	}
	if age > runtimeinstance.StaleAfter {
		return "stale", ageSeconds
	}
	return "active", ageSeconds
}
