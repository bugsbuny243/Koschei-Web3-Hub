package defense

import (
	"testing"
	"time"
)

func TestRunShadowResolvesProgramSurfaceWithoutVerdictAuthority(t *testing.T) {
	now := time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC)
	source := map[string]any{
		"lp_control": map[string]any{
			"pool_program": "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA",
		},
		"final_verdict": map[string]any{"grade": "D", "signed": true},
	}
	got := RunShadow("Mint111111111111111111111111111111111111111", "solana-mainnet", source, now)
	if !got.Enabled || !got.ShadowMode || got.ExecutionMode != ModeShadow {
		t.Fatalf("shadow runtime contract missing: %+v", got)
	}
	if got.VerdictAuthority || got.CanExecuteMainnet || got.CanModifySource {
		t.Fatalf("shadow runtime gained forbidden authority: %+v", got)
	}
	if got.Status != RuntimePartial || len(got.ToolInvocations) != 1 {
		t.Fatalf("program surface was not observed: %+v", got)
	}
	invocation := got.ToolInvocations[0]
	if invocation.Status != ToolObserved || invocation.ToolName != "resolve_program_surface" {
		t.Fatalf("unexpected tool invocation: %+v", invocation)
	}
	programs, ok := invocation.Output["program_ids"].([]string)
	if !ok || len(programs) != 1 || programs[0] != "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA" {
		t.Fatalf("program IDs were not resolved: %#v", invocation.Output["program_ids"])
	}
	for _, agent := range got.Agents {
		if agent.VerdictAuthority {
			t.Fatalf("agent %s gained verdict authority", agent.Role)
		}
	}
}

func TestRunShadowIsDeterministicForFixedEvidenceAndTime(t *testing.T) {
	now := time.Date(2026, 7, 19, 4, 5, 0, 0, time.UTC)
	source := map[string]any{
		"source_context": map[string]any{"program_id": "11111111111111111111111111111111"},
	}
	first := RunShadow("Target1111111111111111111111111111111111111", "solana-mainnet", source, now)
	second := RunShadow("Target1111111111111111111111111111111111111", "solana-mainnet", source, now)
	if first.InputHash != second.InputHash || first.ReportHash != second.ReportHash || first.CaseRef != second.CaseRef {
		t.Fatalf("fixed shadow run is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.ToolInvocations[0].ToolRunID != second.ToolInvocations[0].ToolRunID {
		t.Fatalf("tool run ID changed for the same immutable input")
	}
}

func TestRunShadowFailsClosedWithoutProgramEvidence(t *testing.T) {
	got := RunShadow("Target1111111111111111111111111111111111111", "", map[string]any{}, time.Date(2026, 7, 19, 4, 10, 0, 0, time.UTC))
	if got.Status != RuntimeEvidencePending {
		t.Fatalf("missing program evidence should remain pending: %+v", got)
	}
	if len(got.ToolInvocations) != 1 || got.ToolInvocations[0].Status != ToolEvidencePending {
		t.Fatalf("tool result did not preserve missing evidence: %+v", got.ToolInvocations)
	}
	if got.Agents[2].Status != RuntimeBlocked {
		t.Fatalf("reproduction should be blocked before a verified finding: %+v", got.Agents[2])
	}
}

func TestDisabledReportPreservesExistingDeterministicSystem(t *testing.T) {
	got := DisabledReport("Target1111111111111111111111111111111111111", "solana-mainnet", time.Date(2026, 7, 19, 4, 15, 0, 0, time.UTC))
	if got.Enabled || got.ExecutionMode != ModeDisabled || got.Status != RuntimeDisabled {
		t.Fatalf("disabled runtime contract changed: %+v", got)
	}
	if got.VerdictAuthority || got.CanExecuteMainnet || got.CanModifySource {
		t.Fatalf("disabled runtime has forbidden authority: %+v", got)
	}
}

func TestInitialToolRegistryIsReadOnlyAndNonAuthoritative(t *testing.T) {
	registry := InitialToolRegistry()
	if len(registry) < 4 {
		t.Fatalf("initial tool registry is incomplete: %+v", registry)
	}
	for _, tool := range registry {
		if !tool.ReadOnly || tool.CanChangeVerdict {
			t.Fatalf("unsafe initial tool contract: %+v", tool)
		}
	}
}
