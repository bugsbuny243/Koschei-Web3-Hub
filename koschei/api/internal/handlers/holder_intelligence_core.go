package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

// holderIntelligenceCoreResult is the reusable holder-analysis boundary. It
// preserves raw account evidence in Roles while exposing owner-normalized
// concentration, bounded history and launch forensics through Intelligence.
type holderIntelligenceCoreResult struct {
	Request         services.SecurityRadarRequest
	Analysis        services.ArvisAnalysis
	Bundle          services.SecurityRadarBundle
	Arms            []services.SecurityRadarVerdict
	Final           services.SecurityRadarFinalVerdict
	Roles           services.HolderRoleAnalysis
	Distribution    map[string]any
	Cluster         services.HolderClusterAnalysis
	Market          services.TokenMarketSnapshot
	Intelligence    services.HolderIntelligence
	LaunchForensics services.LaunchForensicsAnalysis
	SourceContext   map[string]any
}

func (h *Handler) runHolderIntelligenceCore(parent context.Context, target, network, mode string) holderIntelligenceCoreResult {
	if parent == nil {
		parent = context.Background()
	}
	target = strings.TrimSpace(target)
	network = strings.TrimSpace(network)
	if network == "" {
		network = "solana-mainnet"
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "manual_detail"
	}

	req := services.SecurityRadarRequest{Target: target, Network: network, Mode: mode}
	analysis := services.AnalyzeArvisRadarsContext(parent, req)
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	roles := services.ArvisHolderRolesFromBundle(bundle)
	distribution := radarDetailHolderDistributionFromRoles(roles)
	if !roles.Available {
		distribution, roles = radarDetailHolderDistribution(parent, target)
	}
	cluster := services.ArvisHolderClusterFromBundle(bundle)
	source := h.radarDetailSourceContext(parent, target, network)
	launch := h.analyzeLaunchForensics(parent, target, roles, cluster, source)
	analysis = services.ApplyLaunchForensicsToAnalysis(analysis, req, launch)
	bundle = services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := services.ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}
	final := services.ArvisFinalFromBundle(bundle)
	market := radarDetailMarketSnapshot(parent, target)
	intelligence := services.ApplyLaunchForensicsToHolderIntelligence(
		services.BuildHolderIntelligence(roles, cluster, market, time.Now().UTC()),
		launch,
	)
	if h != nil && h.DB != nil {
		services.NewSecurityRadarStore(h.DB).CaptureLaunchForensicsFloor(parent, target, network, launch)
	}
	return holderIntelligenceCoreResult{
		Request: req, Analysis: analysis, Bundle: bundle, Arms: arms, Final: final,
		Roles: roles, Distribution: distribution, Cluster: cluster, Market: market,
		Intelligence: intelligence, LaunchForensics: launch, SourceContext: source,
	}
}

func holderIntelligenceCoreConcentration(core holderIntelligenceCoreResult) (float64, float64, bool) {
	if core.Intelligence.Available && core.Intelligence.CirculatingSupply > 0 {
		return core.Intelligence.Top1Percentage, core.Intelligence.Top10Percentage, true
	}
	if core.Roles.Available && core.Roles.CirculatingSupply > 0 {
		return core.Roles.EffectiveTop1Percentage, core.Roles.EffectiveTop10Percentage, true
	}
	return 0, 0, false
}

func holderIntelligenceCoreStatus(core holderIntelligenceCoreResult) string {
	if strings.TrimSpace(core.Intelligence.Status) != "" {
		return core.Intelligence.Status
	}
	if strings.TrimSpace(core.Roles.Status) != "" {
		return core.Roles.Status
	}
	return "holder_data_unavailable"
}

func holderIntelligenceCorePolicy(core holderIntelligenceCoreResult) string {
	if !core.Intelligence.Available || core.Intelligence.FinalVerdictBlocked || core.Roles.BlockingEvidenceGap {
		return "withhold"
	}
	return "evidence_backed"
}

func holderIntelligenceCoreEvidence(core holderIntelligenceCoreResult) []string {
	values := []string{}
	values = appendUniqueHolderCoreEvidence(values, core.Intelligence.Findings...)
	values = appendUniqueHolderCoreEvidence(values, core.Cluster.Findings...)
	values = appendUniqueHolderCoreEvidence(values, core.LaunchForensics.Findings...)
	if strings.TrimSpace(core.LaunchForensics.Summary) != "" {
		values = appendUniqueHolderCoreEvidence(values, core.LaunchForensics.Summary)
	}
	for _, limitation := range core.Intelligence.Limitations {
		values = appendUniqueHolderCoreEvidence(values, "LIMITATION: "+limitation)
	}
	for _, limitation := range core.LaunchForensics.Limitations {
		values = appendUniqueHolderCoreEvidence(values, "LIMITATION: "+limitation)
	}
	return values
}

func appendUniqueHolderCoreEvidence(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, value := range dst {
		seen[strings.TrimSpace(value)] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		dst = append(dst, value)
	}
	return dst
}

func holderIntelligenceCoreExplanation(core holderIntelligenceCoreResult) string {
	if !core.Intelligence.Available {
		return "Holder intelligence is unavailable or incomplete; missing history is not treated as a safety signal."
	}
	parts := []string{}
	if core.Intelligence.OwnerAggregationApplied {
		parts = append(parts, fmt.Sprintf(
			"Token accounts were resolved and aggregated into %d controlling owner surfaces before concentration was calculated.",
			core.Intelligence.OwnerCount,
		))
	}
	if protocol := core.Roles.ProtocolControlledPercentage + core.Roles.BurnPercentage; protocol > 0 {
		parts = append(parts, fmt.Sprintf(
			"Positively identified protocol, liquidity or burn inventory representing %.4f%% of raw supply was reported separately from ordinary holder risk.",
			protocol,
		))
	}
	parts = append(parts, fmt.Sprintf(
		"Ordinary risk-bearing owners account for an owner-normalized Top 1 concentration of %.4f%% and Top 10 concentration of %.4f%%.",
		core.Intelligence.Top1Percentage,
		core.Intelligence.Top10Percentage,
	))
	if core.Cluster.WalletsRequested > 0 {
		parts = append(parts, fmt.Sprintf(
			"Bounded behavior observation used deep history for %d owners and shallow history for %d owners, consuming %d of the %d-call RPC budget.",
			core.Cluster.DeepOwnersScanned,
			core.Cluster.ShallowOwnersScanned,
			core.Cluster.RPCCallsUsed,
			core.Cluster.RPCBudget,
		))
	}
	limited := 0
	for _, row := range core.Intelligence.Rows {
		if row.ObservationBudgetDegraded || row.ObservationStatus == "rpc_budget_exhausted" ||
			row.ObservationStatus == "no_observed_signatures" || row.ObservationStatus == "signature_only_observation" {
			limited++
		}
	}
	if limited > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d owner observations were limited, signature-only, empty within the queried window or budget-degraded; those gaps are not classified as safe or organic.",
			limited,
		))
	}
	if strings.TrimSpace(core.LaunchForensics.Summary) != "" {
		parts = append(parts, core.LaunchForensics.Summary)
	}
	if core.Intelligence.FinalVerdictBlocked {
		parts = append(parts, "A dominant owner role remains unresolved, so the holder verdict is withheld rather than downgraded.")
	}
	return strings.Join(parts, " ")
}

type customerTokenScanResult struct {
	web3.TokenRiskResult
	HolderDistribution   map[string]any                   `json:"holder_distribution"`
	HolderIntelligence   services.HolderIntelligence      `json:"holder_intelligence"`
	HolderCluster        services.HolderClusterAnalysis   `json:"holder_cluster"`
	LaunchForensics      services.LaunchForensicsAnalysis `json:"launch_forensics"`
	VerifiedEvidence     []string                         `json:"verified_evidence"`
	Explanation          string                           `json:"explanation"`
	HolderAnalysisStatus string                           `json:"holder_analysis_status"`
	FinalPolicy          string                           `json:"final_policy"`
	VerdictWithheld      bool                             `json:"verdict_withheld"`
}

func (h *Handler) scanCustomerToken(ctx context.Context, network, mint string) (customerTokenScanResult, error) {
	base, err := h.tokenService().ScanToken(ctx, network, mint)
	if err != nil {
		return customerTokenScanResult{}, err
	}
	core := h.runHolderIntelligenceCore(ctx, mint, network, "customer_token_scan")
	return applyHolderCoreToTokenRisk(base, core), nil
}

func applyHolderCoreToTokenRisk(base web3.TokenRiskResult, core holderIntelligenceCoreResult) customerTokenScanResult {
	// The legacy raw token-account percentages are never authoritative once this
	// mapper runs. Only owner-normalized values from the shared core are scored.
	base.Token.LargestHolderPercent = 0
	base.Token.TopTenPercent = 0
	if top1, top10, ok := holderIntelligenceCoreConcentration(core); ok {
		base.Token.LargestHolderPercent = roundPercent(top1)
		base.Token.TopTenPercent = roundPercent(top10)
	}
	rescored := web3.ScoreTokenRisk(base.Token)
	if strings.TrimSpace(base.Disclaimer) != "" {
		rescored.Disclaimer = base.Disclaimer
	}
	rescored.Findings = appendUniqueHolderCoreEvidence(rescored.Findings, holderIntelligenceCoreEvidence(core)...)
	policy := holderIntelligenceCorePolicy(core)
	if policy == "withhold" {
		rescored.Findings = appendUniqueHolderCoreEvidence(rescored.Findings,
			"Holder verdict withheld: unresolved or incomplete holder evidence is not a low-risk signal.",
		)
	}
	return customerTokenScanResult{
		TokenRiskResult:      rescored,
		HolderDistribution:   core.Distribution,
		HolderIntelligence:   core.Intelligence,
		HolderCluster:        core.Cluster,
		LaunchForensics:      core.LaunchForensics,
		VerifiedEvidence:     holderIntelligenceCoreEvidence(core),
		Explanation:          holderIntelligenceCoreExplanation(core),
		HolderAnalysisStatus: holderIntelligenceCoreStatus(core),
		FinalPolicy:          policy,
		VerdictWithheld:      policy == "withhold",
	}
}

func holderIntelligenceCoreShieldAction(core holderIntelligenceCoreResult) string {
	if holderIntelligenceCorePolicy(core) == "withhold" || !core.Final.Signed {
		return "withhold"
	}
	return shieldAction(core.Final.RiskLevel, core.Final.RiskIndex)
}
