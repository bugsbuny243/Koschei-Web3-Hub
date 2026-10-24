package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"
)

func TestBackgroundRuntimeWiresPreservedProductionWorkers(t *testing.T) {
	// Parse the file so commented-out calls do not count as wiring.
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "runtime_workers.go", nil, 0)
	if err != nil {
		t.Fatalf("parse runtime worker wiring: %v", err)
	}
	calls := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, call); err == nil {
			calls[buf.String()] = true
		}
		return true
	})
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
		"handlers.StartCanonicalArvisStreamJobScheduler(ctx, db, jobStore, runtimeHealth)",
	} {
		if !calls[required] {
			t.Fatalf("production background runtime missing preserved worker call %q", required)
		}
	}
}
