package services

import "testing"

func TestArvisHeartbeatSourcesIncludeRaydiumPumpAndPumpSwap(t *testing.T) {
	t.Setenv("RAYDIUM_PROGRAM_ID", "")
	t.Setenv("PUMP_FUN_PROGRAM_ID", "")
	t.Setenv("PUMP_SWAP_PROGRAM_ID", "")

	sources := arvisHeartbeatSources()
	if len(sources) != 3 {
		t.Fatalf("expected 3 heartbeat sources, got %d", len(sources))
	}

	byLabel := map[string]arvisHeartbeatSource{}
	for _, source := range sources {
		if source.ProgramID == "" || source.EventType == "" || source.Label == "" {
			t.Fatalf("incomplete source: %#v", source)
		}
		byLabel[source.Label] = source
	}

	raydium, ok := byLabel["raydium_program"]
	if !ok {
		t.Fatal("Raydium heartbeat source missing")
	}
	if raydium.ModuleID != ModuleRaydiumPoolGuardian {
		t.Fatalf("unexpected Raydium module: %s", raydium.ModuleID)
	}
	if raydium.ProgramID != "675kPX9MHTjS2zt1qfr1NYhd1B9M9QGK6cEcDDCo2t9" {
		t.Fatalf("unexpected Raydium program id: %s", raydium.ProgramID)
	}

	pump, ok := byLabel["pump_program"]
	if !ok {
		t.Fatal("Pump heartbeat source missing")
	}
	if pump.ModuleID != ModulePumpSybilRadar || pump.ProgramID != defaultPumpProgramID {
		t.Fatalf("unexpected Pump source: %#v", pump)
	}
	if pump.EventType != "pump_program_signature" {
		t.Fatalf("unexpected Pump event type: %s", pump.EventType)
	}

	pumpSwap, ok := byLabel["pumpswap_program"]
	if !ok {
		t.Fatal("PumpSwap heartbeat source missing")
	}
	if pumpSwap.ModuleID != ModulePumpSybilRadar || pumpSwap.ProgramID != defaultPumpSwapProgramID {
		t.Fatalf("unexpected PumpSwap source: %#v", pumpSwap)
	}
	if pumpSwap.EventType != "pumpswap_program_signature" {
		t.Fatalf("unexpected PumpSwap event type: %s", pumpSwap.EventType)
	}
}

func TestArvisHeartbeatSourcesRespectRenderOverrides(t *testing.T) {
	t.Setenv("RAYDIUM_PROGRAM_ID", "raydium-override")
	t.Setenv("PUMP_FUN_PROGRAM_ID", "pump-override")
	t.Setenv("PUMP_SWAP_PROGRAM_ID", "pumpswap-override")

	sources := arvisHeartbeatSources()
	byLabel := map[string]arvisHeartbeatSource{}
	for _, source := range sources {
		byLabel[source.Label] = source
	}
	if byLabel["raydium_program"].ProgramID != "raydium-override" {
		t.Fatalf("Raydium override ignored: %s", byLabel["raydium_program"].ProgramID)
	}
	if byLabel["pump_program"].ProgramID != "pump-override" {
		t.Fatalf("Pump override ignored: %s", byLabel["pump_program"].ProgramID)
	}
	if byLabel["pumpswap_program"].ProgramID != "pumpswap-override" {
		t.Fatalf("PumpSwap override ignored: %s", byLabel["pumpswap_program"].ProgramID)
	}
}
