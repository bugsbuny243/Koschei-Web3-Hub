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
		"services.StartPumpPortalRadarIfEnabled(ctx, db)",
		"services.StartActorDefenseCorrelator(ctx, db)",
		"handlers.StartWatchlistMonitor(ctx, db)",
		"handlers.StartCanonicalInvestigationJobWorker(ctx, db, readDB, solanaRPC, jobStore)",
		"handlers.StartCanonicalPumpJobScheduler(ctx, db, jobStore)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("production background runtime missing preserved worker wiring %q", required)
		}
	}
}
