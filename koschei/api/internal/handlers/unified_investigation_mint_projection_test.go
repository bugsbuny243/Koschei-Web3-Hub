package handlers

import (
	"context"
	"testing"

	"koschei/api/internal/services"
)

func TestUnifiedInvestigationProjectsVerifiedPYUSDMintEvidence(t *testing.T) {
	const pyusd = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const owner = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	core := holderIntelligenceCoreResult{
		Request: services.SecurityRadarRequest{
			Target: pyusd, Network: "solana-mainnet", Mode: "stored_only_projection",
		},
		SourceContext: map[string]any{
			"mint_owner_program": owner,
			"mint_owner_program_evidence_status": "verified",
			"token_2022": true,
			"token_2022_extension_evidence": map[string]any{
				"status": "resolved", "evidence_complete": true,
				"extension_count": 2,
				"extension_names": []string{"permanentDelegate", "transferHook"},
			},
		},
	}
	report := (&Handler{}).assembleUnifiedInvestigationReport(context.Background(), core).Report
	if report["mint_owner_program"] != owner || report["mint_owner_program_evidence_status"] != "verified" || report["token_2022"] != true {
		t.Fatalf("unified report lost verified Token-2022 mint owner: %#v", report["mint_owner_program_evidence"])
	}
	extensions, ok := report["token_2022_extension_evidence"].(map[string]any)
	if !ok || extensions["status"] != "resolved" || extensions["extension_count"] != 2 || extensions["evidence_complete"] != true {
		t.Fatalf("unified report lost extension observations: %#v", report["token_2022_extension_evidence"])
	}
}

func TestUnifiedInvestigationRejectsUnverifiedMintEvidence(t *testing.T) {
	core := holderIntelligenceCoreResult{
		Request: services.SecurityRadarRequest{
			Target: "unverified", Network: "solana-mainnet", Mode: "stored_only_projection",
		},
		SourceContext: map[string]any{
			"mint_owner_program": "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb",
			"mint_owner_program_evidence_status": "unverified",
			"token_2022": true,
			"token_2022_extension_evidence": map[string]any{
				"status": "resolved", "evidence_complete": true,
				"extension_count": 1, "extension_names": []string{"permanentDelegate"},
			},
		},
	}
	report := (&Handler{}).assembleUnifiedInvestigationReport(context.Background(), core).Report
	if report["mint_owner_program_evidence_status"] != "unverified" {
		t.Fatalf("unverified mint owner was promoted: %v", report["mint_owner_program_evidence_status"])
	}
	if _, ok := report["mint_owner_program"]; ok {
		t.Fatalf("untrusted mint program leaked into verified top-level report: %v", report["mint_owner_program"])
	}
	if _, ok := report["token_2022"]; ok {
		t.Fatalf("untrusted Token-2022 classification was published: %v", report["token_2022"])
	}
	evidence, ok := report["token_2022_extension_evidence"].(map[string]any)
	if !ok || evidence["status"] != "unverified" || evidence["evidence_complete"] != false {
		t.Fatalf("untrusted extensions promoted to verified: %#v", evidence)
	}
}
