package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"koschei/api/internal/alerts"
	"koschei/api/internal/cryptobrief"
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
		runtimeHealth.Register("arvis-telegram-delivery", "worker", "", false)
		return func() {}
	}

	stops := make([]func(), 0, 12)
	for _, cfg := range campaignConfigs {
		if cfg != nil {
			stops = append(stops, services.StartGlobalCampaignRuntime(ctx, *cfg))
			if globalCampaignReplaySource != nil {
				stops = append(stops, services.StartGlobalCampaignReconciler(ctx, cfg.DB, globalCampaignReplaySource, cfg.Health))
			}
			if globalCampaignEvidenceSink != nil {
				stops = append(stops, services.StartGlobalCampaignEvidenceBridge(ctx, cfg.DB, globalCampaignEvidenceSink, cfg.Health))
			}
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

		arvisTelegram := cryptobrief.New(db)
		if jobStore != nil {
			jobStore.SetCompletionHook(func(hookCtx context.Context, job jobs.Job, result any) error {
				if strings.TrimSpace(job.Type) != handlers.CanonicalInvestigationJobType || strings.TrimSpace(job.UserID) == "" {
					return nil
				}
				var request struct {
					Mode string `json:"mode"`
				}
				if len(job.RequestPayload) > 0 && string(job.RequestPayload) != "null" {
					_ = json.Unmarshal(job.RequestPayload, &request)
				}
				// Recursive/system investigations remain internal evidence. Telegram
				// mirrors customer-requested scan results, not every background branch.
				if strings.TrimSpace(request.Mode) != "customer_canonical_job" {
					return nil
				}
				envelope, ok := result.(map[string]any)
				if !ok {
					encoded, err := json.Marshal(result)
					if err != nil {
						return err
					}
					envelope = map[string]any{}
					if err := json.Unmarshal(encoded, &envelope); err != nil {
						return err
					}
				}
				_, err := arvisTelegram.QueueARVISResult(hookCtx, job.UserID, job.ID, job.Target, job.Network, envelope)
				return err
			})
		}

		stops = append(stops,
			cryptobrief.Start(ctx, db, runtimeHealth),
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
