package services

import (
	"testing"
	"time"
)

func TestClaimSurfaceRejectsPlainText(t *testing.T) {
	evidence := parseArvisClaimSurface("this is not a url")
	if evidence.Available {
		t.Fatal("plain text must not become URL evidence")
	}
}

func TestSafeHTTPSClaimSurfaceProducesVerifiedMonitorEvidence(t *testing.T) {
	evidence := parseArvisClaimSurface("https://example.com/claim?id=42")
	if !evidence.Available || !evidence.HTTPS || evidence.IPLiteralHost || evidence.PunycodeHost {
		t.Fatalf("unexpected parsed evidence: %#v", evidence)
	}
	req := SecurityRadarRequest{Target: evidence.Original, Network: "solana-mainnet"}
	arm := buildClaimSurfaceArm(req, evidence, time.Now().UTC().Format(time.RFC3339))
	if !SecurityRadarVerdictHasVerifiedEvidence(arm) {
		t.Fatal("valid parsed URL evidence must be verified")
	}
	if arm.RiskIndex >= 35 {
		t.Fatalf("ordinary HTTPS surface should remain monitor-level, got %d", arm.RiskIndex)
	}
	if onchain, _ := arm.Signals["real_onchain_evidence"].(bool); onchain {
		t.Fatal("URL parser evidence must not be labeled on-chain")
	}
	if offchain, _ := arm.Signals["real_offchain_evidence"].(bool); !offchain {
		t.Fatal("URL parser evidence must be labeled off-chain")
	}
}

func TestSuspiciousClaimSurfaceProducesHighRiskEvidence(t *testing.T) {
	raw := "http://192.0.2.7:8080/airdrop/claim?seedphrase=alpha&approve_transaction=1&redirect_uri=https%3A%2F%2Fexample.org"
	evidence := parseArvisClaimSurface(raw)
	if !evidence.Available || !evidence.IPLiteralHost || evidence.HTTPS {
		t.Fatalf("unexpected suspicious evidence: %#v", evidence)
	}
	req := SecurityRadarRequest{Target: raw, Network: "solana-mainnet"}
	shield := buildWalletlessClaimArm(req, evidence, time.Now().UTC().Format(time.RFC3339))
	surface := buildClaimSurfaceArm(req, evidence, time.Now().UTC().Format(time.RFC3339))
	if !shield.Signed || !surface.Signed {
		t.Fatal("parsed suspicious surfaces must produce signed structural evidence")
	}
	if shield.RiskIndex < 65 || surface.RiskIndex < 65 {
		t.Fatalf("expected high risk, got shield=%d surface=%d", shield.RiskIndex, surface.RiskIndex)
	}
}

func TestVerifiedFinalArmAcceptsOffchainEvidence(t *testing.T) {
	req := SecurityRadarRequest{Target: "https://example.com/claim", Network: "solana-mainnet"}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	claim := verifiedEvidenceArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, 44, map[string]any{
		"verified_evidence": true,
		"real_offchain_evidence": true,
		"real_onchain_evidence": false,
	}, []string{"parsed URL"}, generatedAt)
	missing := unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "mint required")
	final := buildVerifiedFinalArm(req, []SecurityRadarVerdict{claim, missing}, generatedAt)
	if !final.Signed || final.RiskIndex != 44 {
		t.Fatalf("unexpected final verdict: %#v", final)
	}
	if onchain, _ := final.Signals["real_onchain_evidence"].(bool); onchain {
		t.Fatal("off-chain-only final verdict must not claim on-chain evidence")
	}
	if offchain, _ := final.Signals["real_offchain_evidence"].(bool); !offchain {
		t.Fatal("off-chain final evidence flag missing")
	}
}
