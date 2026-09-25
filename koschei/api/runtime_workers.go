package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"koschei/api/internal/handlers"
	"koschei/api/internal/jobs"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

type runtimeRole string

const (
	runtimeRoleCombined runtimeRole = "combined"
	runtimeRoleAPI      runtimeRole = "api"
	runtimeRoleWorker   runtimeRole = "worker"
)

func parseRuntimeRole(raw string) (runtimeRole, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(runtimeRoleCombined):
		return runtimeRoleCombined, nil
	case string(runtimeRoleAPI):
		return runtimeRoleAPI, nil
	case string(runtimeRoleWorker):
		return runtimeRoleWorker, nil
	default:
		return "", fmt.Errorf("supported values are %q, %q, or %q", runtimeRoleCombined, runtimeRoleAPI, runtimeRoleWorker)
	}
}

func (r runtimeRole) servesHTTP() bool {
	return r == runtimeRoleCombined || r == runtimeRoleAPI
}

func (r runtimeRole) runsBackgroundWorkers() bool {
	return r == runtimeRoleCombined || r == runtimeRoleWorker
}

func startBackgroundRuntime(
	ctx context.Context,
	role runtimeRole,
	db, readDB *sql.DB,
	solanaRPC *web3.SolanaRPC,
	jobStore *jobs.Store,
	globalRadarBackground *services.GlobalRadarBackgroundTelemetryConfig,
	globalRadarHeadIngest *services.GlobalRadarHeadIngestConfig,
) func() {
	if !role.runsBackgroundWorkers() {
		return func() {}
	}

	stops := make([]func(), 0, 6)
	if globalRadarBackground != nil {
		stops = append(stops, services.StartGlobalRadarBackgroundTelemetry(ctx, *globalRadarBackground))
	}
	if globalRadarHeadIngest != nil {
		stops = append(stops, services.StartGlobalRadarHeadIngest(ctx, *globalRadarHeadIngest))
	}
	if db == nil {
		log.Printf("PostgreSQL-backed background runtime not started: APP_DATABASE_URL is not configured")
	} else {
		stops = append(stops,
			services.StartSecurityRadarWatcher(ctx, db, solanaRPC),
			services.StartSecurityRadarSovereignStreamIfEnabled(ctx, db),
			handlers.StartCanonicalInvestigationJobWorker(ctx, db, readDB, solanaRPC, jobStore),
			handlers.StartCanonicalPumpJobScheduler(ctx, db, jobStore),
		)
	}
	return func() {
		for i := len(stops) - 1; i >= 0; i-- {
			if stops[i] != nil {
				stops[i]()
			}
		}
	}
}
