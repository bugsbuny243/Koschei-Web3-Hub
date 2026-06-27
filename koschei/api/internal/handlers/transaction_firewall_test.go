package handlers

import (
	"encoding/base64"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestValidateFirewallTransactionAcceptsBase64(t *testing.T) {
	raw := strings.Repeat("a", 100)
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	if message := validateFirewallTransaction(encoded, "base64"); message != "" {
		t.Fatalf("expected valid transaction, got %q", message)
	}
}

func TestValidateFirewallTransactionRejectsInvalidInput(t *testing.T) {
	if message := validateFirewallTransaction("not-base64", "base64"); message == "" {
		t.Fatal("expected invalid base64 to be rejected")
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("short"))
	if message := validateFirewallTransaction(encoded, "base64"); message == "" {
		t.Fatal("expected short transaction to be rejected")
	}
}

func TestAssessTransactionSimulationBlocksFailedSimulation(t *testing.T) {
	simulation := services.SolanaSimulationResult{}
	simulation.Value.Err = map[string]any{"InstructionError": []any{0, "Custom"}}
	simulation.Value.Logs = []string{"Program Example111111111111111111111111111111 invoke [1]"}

	assessment := assessTransactionSimulation(simulation)
	if assessment.Action != "block" {
		t.Fatalf("expected block, got %s", assessment.Action)
	}
	if assessment.RiskIndex != 100 {
		t.Fatalf("expected risk 100, got %d", assessment.RiskIndex)
	}
}

func TestAssessTransactionSimulationWarnsOnAuthorityChange(t *testing.T) {
	units := int64(200000)
	simulation := services.SolanaSimulationResult{}
	simulation.Value.UnitsConsumed = &units
	simulation.Value.Logs = []string{
		"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [1]",
		"Program log: Instruction: SetAuthority",
	}

	assessment := assessTransactionSimulation(simulation)
	if assessment.Action != "warn" {
		t.Fatalf("expected warn, got %s", assessment.Action)
	}
	if assessment.RiskLevel != "medium" {
		t.Fatalf("expected medium, got %s", assessment.RiskLevel)
	}
	if len(assessment.Findings) != 1 || assessment.Findings[0].Code != "authority_change" {
		t.Fatalf("unexpected findings: %#v", assessment.Findings)
	}
}

func TestAssessTransactionSimulationAllowsLowRiskLogs(t *testing.T) {
	units := int64(120000)
	simulation := services.SolanaSimulationResult{}
	simulation.Value.UnitsConsumed = &units
	simulation.Value.Logs = []string{
		"Program 11111111111111111111111111111111 invoke [1]",
		"Program 11111111111111111111111111111111 success",
	}

	assessment := assessTransactionSimulation(simulation)
	if assessment.Action != "allow" {
		t.Fatalf("expected allow, got %s", assessment.Action)
	}
	if assessment.RiskIndex != 0 {
		t.Fatalf("expected risk 0, got %d", assessment.RiskIndex)
	}
}

func TestAssessTransactionSimulationWithholdsWithoutLogs(t *testing.T) {
	assessment := assessTransactionSimulation(services.SolanaSimulationResult{})
	if assessment.Action != "withhold" {
		t.Fatalf("expected withhold, got %s", assessment.Action)
	}
}
