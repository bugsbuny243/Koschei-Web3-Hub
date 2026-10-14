package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"koschei/api/internal/web3"
)

// AnalyzeArvisRadarsContextIsolated keeps foreground investigations from
// sharing one short RPC deadline across every collector. Background polling
// retains the existing bounded path; interactive/manual/customer scans use
// independent stage contexts so one rate-limited collector cannot consume the
// time reserved for subsequent evidence arms.
func AnalyzeArvisRadarsContextIsolated(ctx context.Context, req SecurityRadarRequest) ArvisAnalysis {
	if ctx == nil {
		ctx = context.Background()
	}
	if !useStageIsolatedRadarScan(ctx, req) {
		return AnalyzeArvisRadarsContext(ctx, req)
	}

	req.Target = strings.TrimSpace(req.Target)
	req.Network = strings.TrimSpace(req.Network)
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Network == "" {
		req.Network = "solana-mainnet"
	}
	if req.Mode == "" {
		req.Mode = SecurityRadarWatchMode
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	// Reserve 20 seconds for response serialization and transport before the 210s client abort.
	const interactiveStageCeiling = 190 * time.Second
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > interactiveStageCeiling {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, interactiveStageCeiling)
		defer cancel()
	}
	profile := collectRadarEvidenceContextIsolated(ctx, req)
	// The same bounded context remains active for the entire isolated analyzer,
	// including arm construction and policy evaluation below.
	sourceModule := arvisSourceModule(req.Mode)

	pump := buildPumpProgramApplicabilityArm(req, profile, generatedAt)
	raydium := buildRaydiumProgramApplicabilityArm(req, profile, generatedAt)
	claimShield := unavailableArm("Walletless Claim Shield", ModuleWalletlessClaimShield, req, generatedAt, "A parsed claim instruction is required; token-holder evidence is not a claim-surface substitute.")
	mev := unavailableArm("MEV Shield", ModuleMEVShield, req, generatedAt, "A signed transaction, route and pool context are required for MEV analysis.")
	authority := buildAuthorityArm(req, profile, generatedAt)
	holders := buildHolderArm(req, profile, generatedAt)
	liquidity := unavailableArm("Liquidity Movement", ModuleLiquidityMovement, req, generatedAt, "Pool reserve or market-liquidity evidence has not been attached yet.")
	creator := unavailableArm("Creator Link Analysis", ModuleCreatorLinkAnalysis, req, generatedAt, "Creator/deployer evidence has not been attached yet.")
	funding := buildFundingClusterArm(req, profile, generatedAt)
	launchDistribution := unavailableArm("Launch Distribution", ModuleLaunchDistribution, req, generatedAt, "Mint-specific ATA initial-recipient evidence has not been attached yet.")
	repeatActors := unavailableArm("Repeat Actor Scan", ModuleRepeatActorScan, req, generatedAt, "Persistent creator/holder actor-index evidence has not been attached yet.")
	sniper := buildSniperTimingArm(req, profile, generatedAt)
	claimSurface := unavailableArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, generatedAt, "URL/domain evidence is required; claim-instruction evidence belongs to Walletless Claim Shield.")
	program := buildProgramRelationArm(req, profile, generatedAt)

	arms := []SecurityRadarVerdict{
		pump, raydium, claimShield, mev, authority, holders, liquidity,
		creator, funding, launchDistribution, repeatActors, sniper,
		claimSurface, program,
	}
	arms = applyRuntimeSecurityModulePolicy(req, generatedAt, arms)
	graph := buildIntelligenceGraphArm(req, profile, generatedAt)
	if !runtimeModuleEnabledForArvisGraph() {
		graph = unavailableArm("Intelligence Graph", ModuleIntelligenceGraph, req, generatedAt, "Module disabled by KOSCHEI_SECURITY_MODULES runtime policy.")
	}
	final := arvisCompatibilityFinal()
	verified := verifiedArvisEvidenceCount(arms)

	summary := SecurityRadarInsufficientEvidenceMessage
	if verified > 0 {
		summary = fmt.Sprintf("ARVIS collected evidence from %d of 14 single-responsibility arms. Letter grade is produced only by EvaluateUnifiedRadarVerdict.", verified)
	}
	bundle := SecurityRadarBundle{
		Target: req.Target, Network: req.Network, Provider: SecurityRadarProvider, WatchMode: req.Mode,
		PumpSybilRadar: pump, RaydiumPoolGuardian: raydium, WalletlessClaimShield: claimShield,
		CustomerSummary: summary, CustomerRecommendation: "evaluate_unified_rules",
		Metadata: map[string]any{
			"brand": "KOSCHEİ WEB3", "sub_product": "ARVIS", "mode": req.Mode,
			"provider": SecurityRadarProvider, "watch_mode": req.Mode, "rule_version": SecurityRadarRuleVersion,
			"architecture_arm_count": 14, "evidence_arm_count": 14, "verified_arm_count": verified,
			"runtime_arm_count": verified, "arvis_arms": arms, "source_module": sourceModule,
			"intelligence_graph":             graph,
			"graph_is_presentation_layer":    true,
			"final_verdict_source":           "EvaluateUnifiedRadarVerdict",
			"numeric_arm_scoring_disabled":   true,
			"investigation_capabilities":     ArvisInvestigationCapabilities(),
			"investigation_capability_scope": ArvisCapabilityRulesetScope,
			"universal_adapter_profiles":     UniversalInvestigationAdapterProfiles(),
			"universal_target_kinds":         UniversalInvestigationTargetKinds(),
			"universal_adapter_policy":       "target-first, chain-family-specific evidence adapters; unsupported or missing evidence remains unknown",
			"data_quality":                   profile.DataQuality,
			"evidence_status":                profile.EvidenceStatus,
			"holder_cluster_analysis":        profile.HolderCluster,
			"funding_recurrence":             profile.FundingRecurrence,
			"collector_deadline_policy":      "independent_stage_deadlines",
		},
	}
	return ArvisAnalysis{Bundle: bundle, Arms: arms, Graph: graph, Final: final}
}

// runtimeModuleEnabledForArvisGraph is kept in this file so the isolated path
// remains behavior-identical to the canonical analyzer without weakening the
// runtime module policy.
func runtimeModuleEnabledForArvisGraph() bool {
	return arvisGraphRuntimeEnabled()
}

func useStageIsolatedRadarScan(ctx context.Context, req SecurityRadarRequest) bool {
	if interactiveSolanaRPCBudget(ctx) {
		return true
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	for _, marker := range []string{"owner", "manual", "customer", "court", "detail", "exposure"} {
		if strings.Contains(mode, marker) {
			return true
		}
	}
	return false
}

func radarEvidenceStageTimeout(req SecurityRadarRequest, stage string) time.Duration {
	if !useStageIsolatedRadarScan(context.Background(), req) {
		return 6500 * time.Millisecond
	}
	budgets := LoadArvisScanBudgets()
	switch stage {
	case "holder_roles", "holder_cluster":
		return time.Duration(budgets.WalletTimeoutSeconds) * time.Second
	case "signatures":
		return time.Duration(budgets.LaunchTimeoutSeconds) * time.Second
	default:
		return 18 * time.Second
	}
}

func radarEvidenceStageContext(parent context.Context, req SecurityRadarRequest, stage string) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timeout := radarEvidenceStageTimeout(req, stage)
	if interactiveSolanaRPCBudget(parent) && timeout < 18*time.Second {
		timeout = 18 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}

func collectRadarEvidenceContextIsolated(parent context.Context, req SecurityRadarRequest) radarEvidenceProfile {
	if parent == nil {
		parent = context.Background()
	}
	profile := radarEvidenceProfile{Target: req.Target, Network: req.Network, DataQuality: "no_rpc_evidence", EvidenceStatus: "insufficient_evidence"}
	rpcURL := strings.TrimSpace(os.Getenv("SOLANA_RPC_URL"))
	if rpcURL == "" {
		rpcURL = web3.SolanaRPCURL(req.Network, os.Getenv("ALCHEMY_API_KEY"))
	}
	if rpcURL == "" {
		profile.Errors = append(profile.Errors, "provider_not_configured: no Solana RPC endpoint configured")
		return profile
	}
	profile.RPCConfigured = true
	if strings.TrimSpace(req.Target) == "" {
		profile.Errors = append(profile.Errors, "target is empty")
		return profile
	}

	accountCtx, accountCancel := radarEvidenceStageContext(parent, req, "account")
	account, accountErr := SolanaGetAccountInfoJSONParsed(accountCtx, rpcURL, req.Target)
	appendStageDeadlineLimitation(&profile, "account", accountErr, accountCtx.Err())
	accountCancel()
	if accountErr == nil && account.Value != nil {
		profile.LiveRPC = true
		profile.AccountExists = true
		profile.AccountOwner = strings.TrimSpace(account.Value.Owner)
		profile.AccountExecutable = account.Value.Executable
		applyParsedMintInfo(&profile, account.Value.Data)
	} else if accountErr != nil {
		profile.Errors = append(profile.Errors, compactRadarError("getAccountInfo", accountErr))
	}

	supplyCtx, supplyCancel := radarEvidenceStageContext(parent, req, "supply")
	supply, supplyErr := SolanaGetTokenSupply(supplyCtx, rpcURL, req.Target)
	appendStageDeadlineLimitation(&profile, "supply", supplyErr, supplyCtx.Err())
	supplyCancel()
	if supplyErr == nil {
		profile.LiveRPC = true
		profile.IsTokenMint = true
		profile.TokenSupply = solanaTokenFloat(supply.Value)
	} else {
		profile.Errors = append(profile.Errors, compactRadarError("getTokenSupply", supplyErr))
	}

	largestCtx, largestCancel := radarEvidenceStageContext(parent, req, "largest_accounts")
	largest, largestErr := SolanaGetTokenLargestAccounts(largestCtx, rpcURL, req.Target)
	appendStageDeadlineLimitation(&profile, "largest_accounts", largestErr, largestCtx.Err())
	largestCancel()
	if largestErr == nil {
		profile.LiveRPC = true
		profile.IsTokenMint = true
		profile.LargestAccounts = len(largest.Value)
		applyLargestHolderEvidence(&profile, largest.Value)
		profile.RawLargestHolderPct = profile.LargestHolderPct
		profile.RawTop10HolderPct = profile.Top10HolderPct

		rolesCtx, rolesCancel := radarEvidenceStageContext(parent, req, "holder_roles")
		profile.HolderRoles = AnalyzeSolanaHolderRoles(rolesCtx, rpcURL, profile.TokenSupply, largest.Value)
		appendStageDeadlineLimitation(&profile, "holder_roles", rolesCtx.Err())
		rolesCancel()
		if profile.HolderRoles.Available && profile.HolderRoles.RoleAdjusted && !profile.HolderRoles.BlockingEvidenceGap {
			profile.LargestHolderPct = int(math.Round(profile.HolderRoles.EffectiveTop1Percentage))
			profile.Top10HolderPct = int(math.Round(profile.HolderRoles.EffectiveTop10Percentage))
		}
		if profile.HolderRoles.BlockingEvidenceGap {
			profile.DataQuality = "partial_rpc_evidence"
			profile.EvidenceStatus = "dominant_holder_role_unresolved"
		}
	} else {
		profile.Errors = append(profile.Errors, compactRadarError("getTokenLargestAccounts", largestErr))
	}

	signaturesCtx, signaturesCancel := radarEvidenceStageContext(parent, req, "signatures")
	signatures, signaturesErr := SolanaGetSignaturesForAddress(signaturesCtx, rpcURL, req.Target, 100)
	appendStageDeadlineLimitation(&profile, "signatures", signaturesErr, signaturesCtx.Err())
	signaturesCancel()
	if signaturesErr == nil {
		profile.LiveRPC = true
		profile.RecentSignatureCount = len(signatures)
		profile.TargetSignatureHistoryExhausted = len(signatures) < 100
		if len(signatures) > 0 {
			profile.LatestSignature = signatures[0].Signature
			profile.LatestSlot = signatures[0].Slot
		}
		var newest, oldest int64
		for i, sig := range signatures {
			if sig.Err != nil {
				profile.FailedSignatureCount++
			}
			if sig.BlockTime != nil && *sig.BlockTime > 0 {
				if i == 0 || *sig.BlockTime > newest {
					newest = *sig.BlockTime
				}
				if oldest == 0 || *sig.BlockTime < oldest {
					oldest = *sig.BlockTime
					profile.TargetOldestSlot = sig.Slot
				}
			}
		}
		if newest > 0 && oldest > 0 && newest >= oldest {
			profile.TargetSignatureTimingObserved = true
			profile.SignatureWindowSeconds = newest - oldest
			profile.TargetOldestBlockTime = oldest
		}
	} else {
		profile.Errors = append(profile.Errors, compactRadarError("getSignaturesForAddress", signaturesErr))
	}

	if parent.Err() == nil && profile.IsTokenMint && profile.HolderRoles.Available {
		clusterCtx, clusterCancel := radarEvidenceStageContext(parent, req, "holder_cluster")
		profile.HolderCluster = AnalyzeSolanaHolderCluster(clusterCtx, rpcURL, req.Target, profile.HolderRoles, profile.TargetOldestBlockTime, profile.TargetOldestSlot)
		appendStageDeadlineLimitation(&profile, "holder_cluster", clusterCtx.Err())
		clusterCancel()
		if store := securityRadarStoreFromContext(parent); store != nil {
			_ = store.CaptureFundingClusters(parent, req.Target, req.Network, profile.HolderCluster)
			if recurrence, err := store.LoadFundingRecurrence(parent, req.Target, req.Network, profile.HolderCluster); err == nil {
				profile.FundingRecurrence = recurrence
			} else {
				profile.FundingRecurrence = FundingRecurrenceAnalysis{
					Status: "unavailable", EvidenceStatus: "unavailable", CurrentTarget: req.Target, Network: normalizeRadarNetwork(req.Network),
					Sources: []FundingSourceRecurrence{}, Limitations: []string{"Funding corpus read failed after holder-cluster persistence."},
				}
			}
		}
	}

	if profile.LiveRPC {
		profile.DataQuality = "live_rpc_evidence"
		profile.EvidenceStatus = "verified_rpc_observation"
		if !profile.IsTokenMint {
			profile.DataQuality = "partial_rpc_evidence"
			profile.EvidenceStatus = "target_not_confirmed_as_token_mint"
		}
	}
	if hasStageDeadlineLimitation(profile.Errors) && profile.LiveRPC {
		profile.DataQuality = "partial_rpc_evidence"
		profile.EvidenceStatus = "partial_rpc_evidence_deadline"
	}
	if parent.Err() != nil {
		profile.Errors = append(profile.Errors, "interactive investigation deadline reached; evidence is partial")
		if profile.LiveRPC {
			profile.DataQuality = "partial_rpc_evidence"
			profile.EvidenceStatus = "partial_rpc_evidence_deadline"
		}
	}
	return profile
}

// appendStageDeadlineLimitation records a named, machine-readable gap even when
// a collector swallows the context error and returns an empty successful value.
func appendStageDeadlineLimitation(profile *radarEvidenceProfile, stage string, errs ...error) {
	if profile == nil {
		return
	}
	for _, err := range errs {
		if errors.Is(err, context.DeadlineExceeded) {
			profile.Errors = append(profile.Errors, "STAGE_DEADLINE_EXCEEDED stage="+stage)
			return
		}
	}
}

func hasStageDeadlineLimitation(errors []string) bool {
	for _, value := range errors {
		if strings.HasPrefix(value, "STAGE_DEADLINE_EXCEEDED stage=") {
			return true
		}
	}
	return false
}
