package handlers

import (
	"context"
	"testing"
)

func TestArvisModuleSourceHealthShapeWithoutDatabase(t *testing.T) {
	moduleID := "pump_sybil_radar"
	result := arvisModuleSourceHealth(context.Background(), nil, moduleID)

	if result["module_id"] != moduleID {
		t.Fatalf("expected module_id %q, got %#v", moduleID, result["module_id"])
	}
	for _, key := range []string{"events", "recent", "enriched", "last_event_at"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("missing stable source-health field %q: %#v", key, result)
		}
	}
}

func TestArvisModuleSourceHealthRejectsEmptyModule(t *testing.T) {
	result := arvisModuleSourceHealth(context.Background(), nil, "")
	if result["events"] != int64(0) || result["recent"] != int64(0) || result["enriched"] != int64(0) {
		t.Fatalf("empty module should return zero counters: %#v", result)
	}
}
