package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"koschei/api/internal/services"
)

func TestUnifiedInvestigationTechnicalResultIsCallerNeutral(t *testing.T) {
	ref := 125000.0
	core := holderIntelligenceCoreResult{
		Request: services.SecurityRadarRequest{Target: "MintParity111111111111111111111111111111111", Network: "solana-mainnet", Mode: "fixture"},
		Arms: []services.SecurityRadarVerdict{
			{Module: "Token Authority Scanner", ModuleID: services.ModuleTokenAuthorityScanner, Signed: true, Signals: map[string]any{"real_onchain_evidence": true, "execution_status": services.ArvisExecutionCompleted, "mint_authority_present": false, "freeze_authority_present": false}, Evidence: []string{"authority parsed"}},
			{Module: "Holder Concentration", ModuleID: services.ModuleHolderConcentration, Signed: true, Signals: map[string]any{"real_onchain_evidence": true, "execution_status": services.ArvisExecutionCompleted, "owner_resolved_top_holder_pct": 36.0}, Evidence: []string{"owner resolved"}},
		},
		Distribution: map[string]any{"available": true, "top_1_percentage": 36.0},
		Market: services.TokenMarketSnapshot{Available: true, Mint: "MintParity111111111111111111111111111111111", LiquidityUSD: 50000},
		Intelligence: services.HolderIntelligence{Available: true, Status: "evidence_backed", CirculatingSupply: 1000000, TopOwnerPercentage: 36, TopOwnerBalance: 360000, TopOwnerReferenceUSDValue: &ref, Rows: []services.HolderIntelligenceRow{}, Findings: []string{}, Limitations: []string{}},
		Cluster: services.HolderClusterAnalysis{Findings: []string{}},
		LaunchForensics: services.LaunchForensicsAnalysis{Available: true, Status: "observed", Findings: []string{}, Limitations: []string{}},
		SourceContext: map[string]any{"creator_wallet": "CreatorParity111111111111111111111111111111"},
	}
	h := &Handler{}
	assembly := h.assembleUnifiedInvestigationReport(context.Background(), core)
	ownerProjection := unifiedInvestigationTechnicalProjection(assembly.Report)
	publicEnvelope := map[string]any{"investigation_report": assembly.Report, "score": 50}
	publicProjection := unifiedInvestigationTechnicalProjection(publicEnvelope["investigation_report"].(map[string]any))
	apiResult := customerTokenScanResult{InvestigationReport: assembly.Report}
	apiProjection := unifiedInvestigationTechnicalProjection(apiResult.InvestigationReport)

	ownerJSON, _ := json.Marshal(ownerProjection)
	publicJSON, _ := json.Marshal(publicProjection)
	apiJSON, _ := json.Marshal(apiProjection)
	if string(ownerJSON) != string(publicJSON) || string(ownerJSON) != string(apiJSON) {
		t.Fatalf("technical investigation differs by caller\nowner=%s\npublic=%s\napi=%s", ownerJSON, publicJSON, apiJSON)
	}
	if _, exists := ownerProjection["generated_at"]; exists {
		t.Fatal("request timestamp leaked into canonical technical projection")
	}
	policy, ok := ownerProjection["investigation_output_policy"].(services.InvestigationOutputPolicy)
	if !ok || !policy.SameTechnicalResult {
		t.Fatalf("caller-neutral policy missing: %#v", ownerProjection["investigation_output_policy"])
	}
}
