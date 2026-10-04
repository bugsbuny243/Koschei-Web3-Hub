package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"koschei/api/internal/alerts"
	"koschei/api/internal/handlers"
	"koschei/api/internal/jobs"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
	"koschei/api/internal/webhooks"
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
	runtimeHealth *runtimehealth.Registry,
	globalRadarBackground *services.GlobalRadarBackgroundTelemetryConfig,
	globalRadarHeadIngest *services.GlobalRadarHeadIngestConfig,
	campaignConfigs ...*services.GlobalCampaignRuntimeConfig,
) func() {
	runtimeHealth.Register(webhooks.DeliveryHealthID, "worker", "", role.runsBackgroundWorkers() && db != nil)
	if !role.runsBackgroundWorkers() {
		return func() {}
	}

	stops := make([]func(), 0, 8)
	for _, cfg := range campaignConfigs {
		if cfg != nil {
			stops = append(stops, services.StartGlobalCampaignRuntime(ctx, *cfg))
		}
	}
	if globalRadarBackground != nil {
		stops = append(stops, services.StartGlobalRadarBackgroundTelemetry(ctx, *globalRadarBackground))
	}
	if globalRadarHeadIngest != nil {
		stops = append(stops, services.StartGlobalRadarHeadIngest(ctx, *globalRadarHeadIngest))
	}
	if db == nil {
		log.Printf("PostgreSQL-backed background runtime not started: APP_DATABASE_URL is not configured")
	} else {
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			stops = append(stops, jobs.StartNATSWakeBridge(ctx, natsURL, os.Getenv("NATS_SUBJECT_PREFIX"), runtimeHealth))
		} else if runtimeHealth != nil {
			runtimeHealth.Register(jobs.NATSWakeHealthID, "worker", "", false)
		}
		stops = append(stops,
			alerts.StartDeliveryWorker(ctx, db),
			webhooks.StartDeliveryWorker(ctx, db, runtimeHealth),
			services.StartGlobalRadarCoverageAlertLifecycle(ctx, db, runtimeHealth),
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
