package services

import "testing"

func TestArvisHeartbeatSourcesIncludeRaydiumAndPump(t *testing.T) {
	t.Setenv("RAYDIUM_PROGRAM_ID", "")
	t.Setenv("PUMP_FUN_PROGRAM_ID", "")

	sources := arvisHeartbeatSources()
	if len(sources) != 2 {
		t.Fatalf("expected 2 heartbeat sources, got %d", len(sources))
	}

	byModule := map[string]arvisHeartbeatSource{}
	for _, source := range sources {
		if source.ProgramID == "" || source.EventType == "" || source.Label == "" {
			t.Fatalf("incomplete source: %#v", source)
		}
		byModule[source.ModuleID] = source
	}

	raydium, ok := byModule[ModuleRaydiumPoolGuardian]
	if !ok {
		t.Fatal("Raydium heartbeat source missing")
	}
	if raydium.ProgramID != "675kPX9MHTjS2zt1qfr1NYhd1B9M9QGK6cEcDDCo2t9" {
		t.Fatalf("unexpected Raydium program id: %s", raydium.ProgramID)
	}

	pump, ok := byModule[ModulePumpSybilRadar]
	if !ok {
		t.Fatal("Pump heartbeat source missing")
	}
	if pump.ProgramID != defaultPumpProgramID {
		t.Fatalf("unexpected Pump program id: %s", pump.ProgramID)
	}
	if pump.EventType != "pump_program_signature" {
		t.Fatalf("unexpected Pump event type: %s", pump.EventType)
	}
}

func TestArvisHeartbeatSourcesRespectRenderOverrides(t *testing.T) {
	t.Setenv("RAYDIUM_PROGRAM_ID", "raydium-override")
	t.Setenv("PUMP_FUN_PROGRAM_ID", "pump-override")

	sources := arvisHeartbeatSources()
	if sources[0].ProgramID != "raydium-override" {
		t.Fatalf("Raydium override ignored: %s", sources[0].ProgramID)
	}
	if sources[1].ProgramID != "pump-override" {
		t.Fatalf("Pump override ignored: %s", sources[1].ProgramID)
	}
}
