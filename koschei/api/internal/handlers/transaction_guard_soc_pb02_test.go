package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"koschei/api/internal/services"
)

func pb02SetAuthorityData(t *testing.T, authorityType byte, revoked bool) []byte {
	t.Helper()
	data := []byte{6, authorityType, 0}
	if !revoked {
		data[2] = 1
		newAuthority := guardV3TestAddress(37)
		raw, err := decodeSolanaPublicKey(newAuthority)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, raw...)
	}
	return data
}

func pb02Fixture(t *testing.T, authorityType byte, revoked bool) (transactionGuardDecodedTransaction, transactionGuardAuthoritySurfaceAnalysis) {
	t.Helper()
	data := pb02SetAuthorityData(t, authorityType, revoked)
	decoded := guardV3AuthorityTestDecoded(guardV3Token2022ProgramID, []string{guardV3TestAddress(35), guardV3TestAddress(36)}, data)
	surface, _ := analyzeTransactionGuardV3AuthoritySurface(decoded, nil, transactionGuardAuthoritySnapshots{})
	return decoded, surface
}

func pb02FullCPI() transactionGuardCPIFlowAnalysis {
	return transactionGuardCPIFlowAnalysis{Requested: true, Required: true, Available: true, Complete: true}
}

func pb02Simulated() transactionFirewallAssessment {
	return transactionFirewallAssessment{SimulationOK: true, Action: "warn", RiskLevel: "high", Findings: []transactionFirewallFinding{}}
}

func TestSOCPreSignPB02DetectsDecodedSetAuthorityAndRevocation(t *testing.T) {
	for _, tc := range []struct {
		name              string
		authorityType     byte
		revoked           bool
		finding, severity string
	}{
		{"mint_tokens", 0, false, "token_authority_mutation_proposed", "critical"},
		{"freeze_account", 1, false, "token_authority_mutation_proposed", "critical"},
		{"account_owner", 2, false, "token_authority_mutation_proposed", "high"},
		{"permanent_delegate", 8, false, "token_authority_mutation_proposed", "critical"},
		{"revocation", 8, true, "no_pb02_authority_mutation_observed", "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, surface := pb02Fixture(t, tc.authorityType, tc.revoked)
			// Authority types 0-3 require a simulated final-state witness;
			// without one the analyzer must withhold rather than assert success.
			if tc.authorityType <= 3 && !surface.Complete {
				withheld := buildTransactionGuardSOCPlaybook02(pb02Simulated(), decoded, pb02FullCPI(), surface)
				if withheld.FindingState != "withheld" || withheld.Severity != "unknown" {
					t.Fatalf("missing final state bypassed PB02: %+v", withheld)
				}
				return
			}
			got := buildTransactionGuardSOCPlaybook02(pb02Simulated(), decoded, pb02FullCPI(), surface)
			if got.PlaybookID != "SOC-PB-02" || got.FindingState != tc.finding ||
				got.Severity != tc.severity || got.EnforcementState != "signer_not_connected" ||
				got.ReportIntegrity != "unsigned_preflight_observation" || len(got.Events) != 1 ||
				got.Events[0].InstructionSource != "outer" {
				t.Fatalf("decoded PB02 outcome not evidence-bound: %+v", got)
			}
		})
	}
}

func TestSOCPreSignPB02DetectsNestedCPISetAuthority(t *testing.T) {
	data := pb02SetAuthorityData(t, 8, false)
	decoded := guardV3AuthorityTestDecoded(guardV3Token2022ProgramID, []string{guardV3TestAddress(35), guardV3TestAddress(36)}, data)
	decoded.parsedInstructions = nil // Only nested CPI contains the mutation.
	decoded.Instructions = []transactionGuardDecodedInstruction{{ProgramID: guardV3TestAddress(38)}}
	groups := []services.SolanaInnerInstructionGroup{{
		Index: 0, Instructions: []services.SolanaInnerInstruction{{
			ProgramIDIndex: 2, Accounts: []int{0, 1}, Data: guardV3Base58Encode(data),
		}},
	}}
	surface, _ := analyzeTransactionGuardV3AuthoritySurface(decoded, groups, transactionGuardAuthoritySnapshots{})
	if !surface.Complete || len(surface.Events) != 1 || surface.Events[0].InstructionSource != "cpi" {
		t.Fatalf("nested authority change was lost: %+v", surface)
	}
	got := buildTransactionGuardSOCPlaybook02(pb02Simulated(), decoded, pb02FullCPI(), surface)
	if got.Severity != "critical" || got.FindingState != "token_authority_mutation_proposed" ||
		len(got.Events) != 1 || got.Events[0].InstructionSource != "cpi" {
		t.Fatalf("nested CPI authority change not surfaced in PB02: %+v", got)
	}
	withoutCPI := buildTransactionGuardSOCPlaybook02(pb02Simulated(), decoded,
		transactionGuardCPIFlowAnalysis{Required: true, Complete: false}, surface)
	if withoutCPI.FindingState != "withheld" || withoutCPI.EvidenceState != "incomplete_or_conflicting" {
		t.Fatalf("missing CPI evidence was promoted into a complete decision: %+v", withoutCPI)
	}
}

func TestSOCPreSignPB02KeepsNormalInstructionsAndMissingEvidenceSeparate(t *testing.T) {
	decoded := transactionGuardDecodedTransaction{Available: true, Complete: true}
	surface := transactionGuardAuthoritySurfaceAnalysis{Available: true, Complete: true}
	benign := buildTransactionGuardSOCPlaybook02(pb02Simulated(), decoded, pb02FullCPI(), surface)
	if benign.FindingState != "no_pb02_authority_mutation_observed" || benign.Severity != "none" ||
		benign.EvidenceState != "decoded_and_simulated" || benign.EnforcementState != "signer_not_connected" {
		t.Fatalf("normal decoded instruction promoted to a threat: %+v", benign)
	}
	bad := pb02Simulated()
	bad.SimulationOK = false
	withheld := buildTransactionGuardSOCPlaybook02(bad, decoded, pb02FullCPI(), surface)
	if withheld.FindingState != "withheld" || withheld.Severity != "unknown" {
		t.Fatalf("simulation failure did not withhold PB02: %+v", withheld)
	}
}

func TestSOCPreSignPB02RoutedResponseNeverClaimsSignerEnforcement(t *testing.T) {
	t.Setenv("TRANSACTION_GUARD_REQUIRE_ENFORCEMENT_PERMIT", "false")
	input, assessment, witness := completeGuardResponseFixture(t)
	decoded, authority := pb02Fixture(t, 8, false)
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/customer/web3/transaction-preflight", nil)
	(&Handler{}).finishTransactionGuardV3ResponseWithWitness(
		rec, request, input, "test-pb02-response", time.Now(),
		assessment, transactionGuardProgramPolicy{Complete: true},
		transactionGuardIntentPolicy{Complete: true},
		decoded, transactionGuardThreatHistoryAnalysis{},
		pb02FullCPI(), authority, witness, "",
	)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	pb02, ok := body["soc_pb02_detection"].(map[string]any)
	if !ok || pb02["severity"] != "critical" || pb02["finding_state"] != "token_authority_mutation_proposed" ||
		pb02["enforcement_state"] != "signer_not_connected" ||
		body["action"] == "allow" || body["enforcement_permit_issued"] == true {
		t.Fatalf("real response path promoted PB02 to an unsafe allow or fake enforcement: %+v", body)
	}
}
