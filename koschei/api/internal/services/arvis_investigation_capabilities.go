package services

const (
	ArvisCapabilityStrong       = "strong_evidence_arm"
	ArvisCapabilityPartial      = "partial_evidence_path"
	ArvisCapabilityPlanned      = "planned_evidence_path"
	ArvisCapabilitySchemaOnly   = "schema_only"
	ArvisCapabilityUnavailable  = "not_a_verified_claim"
	ArvisCapabilityRulesetScope = "actor-v1.0/unified-radar-v1.0"
)

type ArvisInvestigationCapability struct {
	ID                    string   `json:"id"`
	Label                 string   `json:"label"`
	Status                string   `json:"status"`
	PrimaryModules        []string `json:"primary_modules,omitempty"`
	CanonicalSections     []string `json:"canonical_sections"`
	ActorRulesetVersion   string   `json:"actor_ruleset_version"`
	UnifiedRulesetVersion string   `json:"unified_radar_ruleset_version"`
	EvidencePolicy        string   `json:"evidence_policy"`
	NextEvidenceNeed      string   `json:"next_evidence_need,omitempty"`
}

func ArvisInvestigationCapabilities() []ArvisInvestigationCapability {
	return []ArvisInvestigationCapability{
		{
			ID: "solana_token_intelligence", Label: "Solana token intelligence", Status: ArvisCapabilityStrong,
			PrimaryModules:      []string{ModuleTokenAuthorityScanner, ModuleHolderConcentration, ModuleProgramRelationScan},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#2", "ACTOR_INVESTIGATION_ENGINE.md#4"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy: "Token authority, holder concentration and program relation evidence may be signed only when backed by parsed Solana RPC observations.",
		},
		{
			ID: "holder_funding_sybil", Label: "Holder / funding / sybil", Status: ArvisCapabilityStrong,
			PrimaryModules:      []string{ModuleHolderConcentration, ModuleFundingClusterDetector, ModulePumpSybilRadar, ModuleSniperTimingDetector},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#4"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy: "Funding and sybil relations remain evidence rows or watch flags; timing coordination alone is not common ownership.",
		},
		{
			ID: "creator_repeat_actor_memory", Label: "Creator / repeat actor memory", Status: ArvisCapabilityPartial,
			PrimaryModules:      []string{ModuleCreatorLinkAnalysis, ModuleRepeatActorScan},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#2", "ACTOR_INVESTIGATION_ENGINE.md#6"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Creator and repeat-actor claims require persisted actor-index evidence, observed role history or transaction-backed signer evidence.",
			NextEvidenceNeed: "Attach persistent creator/holder actor-index rows to the Repeat Actor Scan without broad recipient wallet-history scans.",
		},
		{
			ID: "launch_sniper_intelligence", Label: "Launch / sniper intelligence", Status: ArvisCapabilityStrong,
			PrimaryModules:      []string{ModuleLaunchDistribution, ModuleSniperTimingDetector},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#2", "ACTOR_INVESTIGATION_ENGINE.md#4"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy: "Launch distribution must stay mint-specific/ATA-based; synchronized launch timing is evidence, not identity attribution.",
		},
		{
			ID: "liquidity_drain_attribution", Label: "Liquidity drain attribution", Status: ArvisCapabilityPartial,
			PrimaryModules:      []string{ModuleLiquidityMovement, ModuleRaydiumPoolGuardian},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#2", "ACTOR_INVESTIGATION_ENGINE.md#5"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Liquidity drain attribution requires parsed add/remove transactions, actor linkage and signed evidence rows before any hard trigger is eligible.",
			NextEvidenceNeed: "Connect pool reserve deltas, LP authority and creator/dominant-holder actor relations to the Liquidity Movement arm.",
		},
		{
			ID: "transaction_intent", Label: "Transaction intent", Status: ArvisCapabilityPartial,
			PrimaryModules:      []string{ModuleProgramRelationScan, ModuleClaimSurfaceRisk, ModuleWalletlessClaimShield},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#1", "ACTOR_INVESTIGATION_ENGINE.md#2", "ACTOR_INVESTIGATION_ENGINE.md#4"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Intent must be derived from parsed instruction, signer, writable account and token/SOL delta evidence, not natural-language guesses.",
			NextEvidenceNeed: "Promote parsed transaction intent fields into one canonical transaction-intent evidence object while preserving the 14-arm contract.",
		},
		{
			ID: "mev_sandwich", Label: "MEV / sandwich", Status: ArvisCapabilityPartial,
			PrimaryModules:      []string{ModuleMEVShield},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#4", "ACTOR_INVESTIGATION_ENGINE.md#5"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Priority, bundle or route exposure can be reported; confirmed sandwich claims require swap, slippage and before/after route evidence.",
			NextEvidenceNeed: "Attach route, slippage and pool-state before/after observations to MEV Shield.",
		},
		{
			ID: "market_manipulation", Label: "Market manipulation", Status: ArvisCapabilityPlanned,
			PrimaryModules:      []string{ModuleFundingClusterDetector, ModuleLiquidityMovement, ModuleHolderConcentration},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#5"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Manipulation labels are not verified claims until deterministic behavior rules cite transaction-backed evidence rows.",
			NextEvidenceNeed: "Map wash/self-flow, coordinated exits and volume/liquidity gaps into versioned deterministic behavior rules.",
		},
		{
			ID: "watch_intelligence", Label: "Watch intelligence", Status: ArvisCapabilityPartial,
			PrimaryModules:      []string{ModuleIntelligenceGraph, ModuleRepeatActorScan},
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#6"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Watch intelligence may surface observed or inferred follow-up needs; it must not silently enable quota-consuming background scans.",
			NextEvidenceNeed: "Connect watchlist observations to durable actor memory without treating inferred evidence as grade input.",
		},
		{
			ID: "cross_chain_intelligence", Label: "Cross-chain intelligence", Status: ArvisCapabilitySchemaOnly,
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#4", "ACTOR_INVESTIGATION_ENGINE.md#6"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "Bridge, chain-hopping, mixer, peel-chain, stablecoin-conversion and CEX/OTC movement claims remain unavailable until verified cross-chain evidence rows exist.",
			NextEvidenceNeed: "Add verified bridge/chain evidence ingestion before surfacing cross-chain criminal-pattern claims.",
		},
		{
			ID: "unverified_cross_chain_crime_patterns", Label: "Cross-chain criminal patterns", Status: ArvisCapabilityUnavailable,
			CanonicalSections:   []string{"ACTOR_INVESTIGATION_ENGINE.md#3", "ACTOR_INVESTIGATION_ENGINE.md#4"},
			ActorRulesetVersion: ActorDefenseRulesetVersion, UnifiedRulesetVersion: UnifiedRadarRulesetVersion,
			EvidencePolicy:   "No mixer, peel-chain, bridge-laundering, OTC or Solana-to-EVM same-actor assertion may be emitted as verified production output without signature-level evidence rows.",
			NextEvidenceNeed: "Define bridge, mixer, peel-chain and stablecoin conversion evidence-row schemas before any production verdict integration.",
		},
	}
}
