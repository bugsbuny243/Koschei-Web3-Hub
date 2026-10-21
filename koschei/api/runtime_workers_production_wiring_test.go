package main

import (
	"os"
	"strings"
	"testing"
)

func TestBackgroundRuntimeWiresPreservedProductionWorkers(t *testing.T) {
	body, err := os.ReadFile("runtime_workers.go")
	if err != nil {
		t.Fatalf("read runtime worker wiring: %v", err)
	}
	source := string(body)
	for _, required := range []string{
		"services.StartGlobalCampaignRuntime(ctx, *cfg)",
		"services.StartGlobalCampaignReconciler(ctx, cfg.DB, globalCampaignReplaySource, cfg.Health)",
		"services.StartGlobalCampaignEvidenceBridge(ctx, cfg.DB, globalCampaignEvidenceSink, cfg.Health)",
		"services.StartGlobalRadarBackgroundTelemetry(ctx, *globalRadarBackground)",
		"services.StartGlobalRadarHeadIngest(ctx, *globalRadarHeadIngest)",
		"jobs.StartNATSWakeBridge(ctx, natsURL, os.Getenv(\"NATS_SUBJECT_PREFIX\"), runtimeHealth)",
		"alerts.StartDeliveryWorker(ctx, db)",
		"webhooks.StartDeliveryWorker(ctx, db, runtimeHealth)",
		"services.StartGlobalRadarCoverageAlertLifecycle(ctx, db, runtimeHealth)",
		"services.StartSecurityRadarSovereignStreamIfEnabled(ctx, db)",
		"services.StartPumpPortalRadarIfEnabled(ctx, db)",
		"services.StartActorDefenseCorrelator(ctx, db)",
		"handlers.StartWatchlistMonitor(ctx, db)",
		"handlers.StartAutopublishWorker(ctx, db)",
		"handlers.StartCanonicalInvestigationJobWorker(ctx, db, readDB, solanaRPC, jobStore)",
		"handlers.StartCanonicalPumpJobScheduler(ctx, db, jobStore)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("production background runtime missing preserved worker wiring %q", required)
		}
	}
}
