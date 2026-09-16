package handlers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const unifiedInvestigationSchemaVersion = "koschei-unified-investigation-v1"

type unifiedInvestigationAssembly struct {
	Report                    map[string]any
	Core                      holderIntelligenceCoreResult
	DB                        *sql.DB
	Store                     *services.ActorDefenseStore
	Creator                   string
	ActorDossier              services.ActorDefenseDossier
	ActorTrack                services.ActorDefenseTrack
	ActorVerdict              services.ActorDefenseRuleVerdict
	CampaignGenome            services.ActorCampaignGenome
	CampaignGenomeSnapshot    services.CampaignGenomeSnapshot
	CampaignGenomePersistence string
	CampaignGenomeMatches     services.CampaignGenomeMatchReport
	OperationalMemory         services.ActorOperationalMemoryReport
	FundingOutcomeMemory      services.FundingClusterOutcomeMemory
	IncidentCorpus            services.SecurityIncidentCorpusView
	ActorIncidentHistory      services.SecurityIncidentCorpusView
	BehavioralSignatures      services.BehavioralSignatureReport
	Behavior                  services.UnifiedRadarBehaviorReport
	UnifiedVerdict            services.UnifiedRadarVerdict
	Threat                    services.ThreatAnticipationReport
	CombinedEvidence          []services.ActorDefenseEvidenceRecord
	Modules                   []map[string]any
	Structural                map[string]any
	Graph                     any
	TradeLedger               map[string]any
	ActorStoreStatus          string
}

type unifiedActorInvestigationRun struct {
	Status                   string                      `json:"status"`
	TriggeredBy              string                      `json:"triggered_by"`
	LiveRequested            bool                        `json:"live_requested"`
	FundingOrigin            services.ActorFundingOrigin `json:"funding_origin"`
	FundingOriginPersistence string                      `json:"funding_origin_persistence"`
	LiveEvidence             actorDefenseLiveCoverage    `json:"live_evidence"`
	RuleVerdictPersistence   string                      `json:"rule_verdict_persistence"`
	Limitations              []string                    `json:"limitations"`
}

// buildUnifiedInvestigationReport runs the shared evidence engine used by public,
// authenticated, owner and API callers. Caller type is intentionally absent from
// the technical result. Operational metadata is added outside Report.
func (h *Handler) buildUnifiedInvestigationReport(ctx context.Context, target, network, mode string) unifiedInvestigationAssembly {
	core := h.runHolderIntelligenceCore(ctx, target, network, mode)
	return h.assembleUnifiedInvestigationReportMode(ctx, core, mode)
}

// assembleUnifiedInvestigationReport preserves the mode embedded by the shared
// holder core. Tests with an empty mode remain stored-only and never call RPC.
func (h *Handler) assembleUnifiedInvestigationReport(ctx context.Context, core holderIntelligenceCoreResult) unifiedInvestigationAssembly {
	mode := strings.TrimSpace(core.Request.Mode)
	if mode == "" {
		mode = "stored_only_projection"
	}
	return h.assembleUnifiedInvestigationReportMode(ctx, core, mode)
}

func (h *Handler) assembleUnifiedInvestigationReportMode(ctx context.Context, core holderIntelligenceCoreResult, mode string) unifiedInvestigationAssembly {
	target := strings.TrimSpace(core.Request.Target)
	network := strings.TrimSpace(core.Request.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	now := time.Now().UTC()
	creator := strings.TrimSpace(creatorIntelCleanString(core.SourceContext["creator_wallet"]))
	liveRequested := unifiedLiveEvidenceAllowed(mode)

	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	actorDossier := services.ActorDefenseDossier{
		Wallet: creator, Network: network,
		Tokens: []services.ActorDefenseTokenObservation{}, RelatedActors: []services.ActorDefenseRelatedActor{},
		Evidence: []services.ActorDefenseEvidenceRecord{}, Coverage: map[string]any{}, Policy: map[string]any{}, GeneratedAt: now,
	}
	actorTrack := services.ActorDefenseTrack{Network: network, TargetKind: "wallet", TargetID: creator, Dossier: map[string]any{}}
	actorStoreStatus := "creator_unavailable"
	creatorRelation := newActorCreatorRelationRun(creator, target)
	distributionRun := newActorDistributionIntegrationRun(creator, target)
	var store *services.ActorDefenseStore
	if db != nil && creator != "" {
		store = services.NewActorDefenseStore(db)
		if liveRequested {
			creatorRelation = h.persistCanonicalCreatorMintRelation(ctx, store, core, creator, network)
		}
		if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 150); err == nil {
			actorDossier, actorTrack, actorStoreStatus = loaded, loaded.Track, "loaded"
		} else {
			actorStoreStatus = "load_failed"
		}
	}

	actorRun := unifiedActorInvestigationRun{
		Status:        "not_requested",
		TriggeredBy:   "creator_discovery",
		LiveRequested: liveRequested,
		FundingOrigin: services.ActorFundingOrigin{
			Wallet: creator, Status: "not_requested", VerificationStatus: "unverified",
			TrailStatus: "not_investigated", IdentityScope: "onchain_wallet_only", Limitations: []string{},
		},
		FundingOriginPersistence: "not_requested",
		LiveEvidence:             actorDefenseLiveCoverage{Status: "not_requested", Limitations: []string{}},
		RuleVerdictPersistence:   "not_requested",
		Limitations:              []string{},
	}
	externalDiscovery := newActorExternalDiscoveryRun(creator)

	// A token scan must not stop after discovering the creator address. Full/live
	// modes invoke creator relation persistence, Solscan discovery, funding origin,
	// wallet evidence and the existing mint-specific distribution investigator.
	// Safe Check and stored-only projections remain network-call free.
	if liveRequested && creator != "" {
		externalDiscovery = h.collectActorExternalDiscovery(ctx, store, creator, network)
		actorRun.Limitations = append(actorRun.Limitations, externalDiscovery.Limitations...)
		actorRun.Limitations = append(actorRun.Limitations, creatorRelation.Limitations...)
	}
	if liveRequested {
		switch {
		case creator == "":
			actorRun.Status = "creator_unavailable"
			actorRun.Limitations = append(actorRun.Limitations, "Token taramasında doğrulanmış creator/deployer cüzdanı çözümlenemedi; actor investigation başlatılmadı.")
		case store == nil:
			actorRun.Status = "database_unavailable"
			actorRun.Limitations = append(actorRun.Limitations, "Actor evidence store kullanılamadığı için creator soruşturması kalıcı olarak çalıştırılamadı.")
		default:
			actorRun.Status = "collecting"
			actorRun.FundingOrigin, actorRun.FundingOriginPersistence = h.collectActorFundingOrigin(ctx, store, creator, network)

			// Solscan observations, current creator relation and funding evidence can
			// enrich the dossier before transaction parsing starts.
			if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 200); err == nil {
				actorDossier, actorTrack = loaded, loaded.Track
				actorStoreStatus = "discovery_funding_refreshed"
			} else {
				actorRun.Limitations = append(actorRun.Limitations, "Discovery ve funding-origin toplandıktan sonra actor dossier yenilenemedi.")
			}

			actorRun.LiveEvidence = h.collectActorDefenseLiveEvidence(ctx, store, actorDossier)
			distributionRun = h.collectCanonicalActorDistribution(ctx, store, creatorRelation, network)
			actorRun.Limitations = append(actorRun.Limitations, distributionRun.Limitations...)
			if loaded, err := store.LoadPersistentWalletDossier(ctx, creator, network, 200); err == nil {
				actorDossier, actorTrack = loaded, loaded.Track
				actorStoreStatus = "live_distribution_refreshed"
			} else {
				actorStoreStatus = "live_refresh_failed"
				actorRun.Limitations = append(actorRun.Limitations, "Canlı actor ve dağıtım kanıtı toplandıktan sonra kalıcı dossier yenilenemedi.")
			}

			switch actorRun.LiveEvidence.Status {
			case "complete":
				actorRun.Status = "complete"
			case "not_requested", "stored_evidence_only":
				actorRun.Status = actorRun.LiveEvidence.Status
			default:
				actorRun.Status = "partial"
			}
			if distributionRun.Status == "partial_persistence" || distributionRun.Status == "creator_mint_relation_unresolved" {
				actorRun.Status = "partial"
			}
		}
	} else if creator != "" && store != nil {
		actorRun.Status = "stored_evidence_only"
		actorRun.FundingOrigin.Status = "stored_evidence_only"
		actorRun.LiveEvidence.Status = "stored_evidence_only"
	}

	actorLifecycle := services.ActorTokenLifecycleRecurrence{
		Status: "not_investigated", EvidenceStatus: "not_investigated", ActorWallet: creator, Network: network, CurrentMint: target,
		OtherMints: []string{}, CreationSignatures: []string{}, CreationSlots: []int64{}, RuggedStatus: "not_classified_by_lifecycle_table", Limitations: []string{},
	}
	if store != nil && creator != "" {
		if loaded, err := store.LoadTokenLifecycleRecurrence(ctx, creator, network, target); err == nil {
			actorLifecycle = loaded
			core.Analysis = services.ApplyActorTokenLifecycleRecurrenceToAnalysis(core.Analysis, loaded)
			core.Bundle = services.EvidenceBackedSecurityRadarBundle(core.Analysis.Bundle)
			core.Arms = services.ArvisArmsFromBundle(core.Bundle)
			if len(core.Arms) == 0 {
				core.Arms = core.Analysis.Arms
			}
			core.Final = services.ArvisFinalFromBundle(core.Bundle)
		} else {
			actorLifecycle.Status = "unavailable"
			actorLifecycle.EvidenceStatus = "unavailable"
			actorLifecycle.Limitations = append(actorLifecycle.Limitations, "Actor lifecycle corpus query failed.")
		}
	}

	actorLifecycle = applyRequestScopeActorLifecycleRecurrence(&core, actorLifecycle, externalDiscovery, creator, network, target)

	actorExit := services.ActorExitRecurrence{
		Status: "not_investigated", EvidenceStatus: "not_investigated", ActorWallet: creator, Network: network, CurrentTarget: target,
		OtherTargets: []string{}, Signatures: []string{}, Slots: []int64{}, EventKinds: []string{}, Events: []services.ActorExitEventReference{}, Limitations: []string{},
	}
	if store != nil && creator != "" {
		if loaded, err := store.LoadActorExitRecurrence(ctx, creator, network, target); err == nil {
			actorExit = loaded
			core.Analysis = services.ApplyActorExitRecurrenceToAnalysis(core.Analysis, loaded)
			core.Bundle = services.EvidenceBackedSecurityRadarBundle(core.Analysis.Bundle)
			core.Arms = services.ArvisArmsFromBundle(core.Bundle)
			if len(core.Arms) == 0 {
				core.Arms = core.Analysis.Arms
			}
			core.Final = services.ArvisFinalFromBundle(core.Bundle)
		} else {
			actorExit.Status = "unavailable"
			actorExit.EvidenceStatus = "unavailable"
			actorExit.Limitations = append(actorExit.Limitations, "Actor exit-event corpus query failed.")
		}
	}

	// Token-scoped live evidence remains a separate collector. Its rows enrich the
	// report but do not silently mutate deterministic rules that require explicit
	// verified actor evidence.
	tradeLedger := h.unifiedTradeLedgerAggregates(ctx, target)
	transactionEvidence := h.loadUnifiedTransactionEvidence(ctx, target, 50)
	liveEvidence := unifiedLiveInvestigationReport{
		Status: "not_requested", Mint: target, WalletCoverage: []unifiedLiveWalletCoverage{},
		Transactions: []unifiedLiveTransactionRow{}, GeneratedAt: now, Limitations: []string{},
		LaunchSigner: unifiedLaunchSignerObservation{Status: "not_requested", InstructionTypes: []string{}, Limitations: []string{}},
	}
	if liveRequested {
		liveEvidence = h.collectUnifiedTokenLiveEvidence(ctx, core)
		transactionEvidence = mergeUnifiedTransactionEvidence(transactionEvidence, unifiedLiveRowsToEvidence(liveEvidence.Transactions))
		if len(transactionEvidence) > 0 {
			tradeLedger = summarizeUnifiedTransactionEvidence(transactionEvidence)
		}
	}

	sales := services.LoadCreatorSellAcceleration(ctx, db, target, creator, now)
	storedVerification := services.CreatorSellVerification{
		CandidateSignatures: append([]string{}, sales.Signatures...), VerifiedSignatures: []string{},
		Limitations: []string{"Acceleration thresholds use the stored trade ledger; live full-scan transaction rows are reported separately and do not rewrite the rule."},
	}
	behavior := services.EvaluateUnifiedRadarBehavior(target, creator, core.Market, core.Intelligence, core.Cluster, sales, now)
	behavior = services.HardenUnifiedRadarBehavior(behavior, storedVerification, core.Cluster)
	behavior = services.ApplyOwnerConcentrationRuleV110(behavior, core.Intelligence, now)
	behavior = services.ApplyCrossTokenFundingRecurrenceRuleV130(behavior, core.FundingRecurrence, now)
	behavior = services.ApplyCrossTokenExitEventRecurrenceRuleV140(behavior, actorExit, now)
	threat := services.BuildThreatAnticipation(services.ThreatAnticipationInput{
		Target: target, Market: core.Market, Holder: core.Intelligence, Cluster: core.Cluster,
		Arms: core.Arms, Behavior: behavior,
	})
	combinedEvidence := append([]services.ActorDefenseEvidenceRecord{}, actorDossier.Evidence...)
	combinedEvidence = append(combinedEvidence, behavior.Evidence...)
	actorVerdict := services.EvaluateActorDefenseRules(actorTrack, combinedEvidence)
	if store != nil && strings.TrimSpace(actorTrack.TargetID) != "" {
		actorRun.RuleVerdictPersistence = "persisted"
		if err := store.PersistRuleVerdict(ctx, actorTrack, actorVerdict); err != nil {
			actorRun.RuleVerdictPersistence = "failed"
			actorRun.Limitations = append(actorRun.Limitations, "Deterministik actor rule verdict kalıcı actor index'e yazılamadı.")
		}
	}

	campaignGenome := services.BuildActorCampaignGenome(actorDossier)
	campaignGenomeSnapshot := services.CampaignGenomeSnapshot{}
	campaignGenomePersistence := "not_eligible"
	if campaignGenome.Complete {
		if liveRequested {
			if db == nil {
				campaignGenomePersistence = "database_unavailable"
			} else if snapshot, inserted, err := services.PersistCampaignGenomeSnapshot(ctx, db, campaignGenome); err != nil {
				campaignGenomePersistence = "failed"
			} else {
				campaignGenomeSnapshot = snapshot
				if inserted {
					campaignGenomePersistence = "persisted"
				} else {
					campaignGenomePersistence = "already_persisted"
				}
			}
		} else {
			campaignGenomePersistence = "not_requested_stored_projection"
		}
	}
	campaignGenomeMatches, campaignGenomeMatchErr := services.LoadCampaignGenomePatternMatches(ctx, db, campaignGenome, 25)
	if campaignGenomeMatchErr != nil {
		campaignGenomeMatches = services.CampaignGenomeMatchReport{
			Version: services.CampaignGenomeIndexSchemaVersion, Network: network, ActorWallet: creator,
			GenomeID: campaignGenome.GenomeID, PatternHashSHA256: campaignGenome.PatternHashSHA256,
			Complete: false, Status: "unavailable", Matches: []services.CampaignGenomePatternMatch{},
			VerdictAuthority: false, SameOperatorClaim: false, RealWorldIdentityClaim: false, WrongdoingClaim: false,
			Limitations: []string{"Campaign genome pattern index could not be read; no cross-wallet genome claim was emitted."},
		}
	}
	operationalMemory, _ := services.LoadActorOperationalMemory(ctx, db, creator, network, 100)
	fundingOutcomeMemory, _ := services.LoadFundingClusterOutcomeMemory(ctx, db, actorRun.FundingOrigin, network, 50)
	incidentCorpus, _ := services.LoadSecurityIncidentCorpus(ctx, db, target, creator, network, 50)
	actorIncidentHistory, _ := services.LoadActorIncidentHistory(ctx, db, creator, network, 50)
	behavioralSignatures, _ := services.LoadBehavioralSignatureReport(ctx, db, target, creator, network, 100)
	modules := unifiedInvestigationModules(core, actorDossier, actorVerdict, campaignGenome, campaignGenomeMatches, operationalMemory, fundingOutcomeMemory, incidentCorpus, actorIncidentHistory, behavioralSignatures, behavior, threat, actorLifecycle, actorExit, creatorRelation, distributionRun, externalDiscovery)
	structural := unifiedStructuralReport(core, actorDossier, actorVerdict, campaignGenome, campaignGenomeMatches, operationalMemory, fundingOutcomeMemory, incidentCorpus, actorIncidentHistory, behavioralSignatures, behavior, threat, actorLifecycle, actorExit, creatorRelation, distributionRun, externalDiscovery)
	graph := unifiedInvestigationGraph(core, actorDossier, actorVerdict, campaignGenome, campaignGenomeMatches, operationalMemory, fundingOutcomeMemory, incidentCorpus, actorIncidentHistory, behavioralSignatures, behavior, threat, actorLifecycle, actorExit, creatorRelation, distributionRun, externalDiscovery)
	unifiedVerdict := services.BuildUnifiedRadarVerdict(services.UnifiedRadarVerdictInput{
		Target: target, Network: network, Market: core.Market, Holder: core.Intelligence, Cluster: core.Cluster,
		Arms: core.Arms, Behavior: behavior, ActorVerdict: actorVerdict, Threat: threat,
	})
	report := map[string]any{
		"schema_version": unifiedInvestigationSchemaVersion,
		"target": target,
		"network": network,
		"mode": mode,
		"generated_at": now,
		"creator": creator,
		"creator_relation": creatorRelation,
		"actor_investigation": actorRun,
		"actor_store_status": actorStoreStatus,
		"actor_lifecycle": actorLifecycle,
		"actor_exit": actorExit,
		"actor_dossier": actorDossier,
		"actor_track": actorTrack,
		"actor_verdict": actorVerdict,
		"campaign_genome": campaignGenome,
		"campaign_genome_snapshot": campaignGenomeSnapshot,
		"campaign_genome_persistence": campaignGenomePersistence,
		"campaign_genome_matches": campaignGenomeMatches,
		"operational_memory": operationalMemory,
		"funding_outcome_memory": fundingOutcomeMemory,
		"incident_corpus": incidentCorpus,
		"actor_incident_history": actorIncidentHistory,
		"behavioral_signatures": behavioralSignatures,
		"behavior": behavior,
		"threat": threat,
		"combined_evidence": combinedEvidence,
		"modules": modules,
		"structural": structural,
		"graph": graph,
		"trade_ledger": tradeLedger,
		"transaction_evidence": transactionEvidence,
		"live_evidence": liveEvidence,
		"unified_verdict": unifiedVerdict,
		"core": core,
	}
	return unifiedInvestigationAssembly{
		Report: report, Core: core, DB: db, Store: store, Creator: creator,
		ActorDossier: actorDossier, ActorTrack: actorTrack, ActorVerdict: actorVerdict,
		CampaignGenome: campaignGenome, CampaignGenomeSnapshot: campaignGenomeSnapshot,
		CampaignGenomePersistence: campaignGenomePersistence, CampaignGenomeMatches: campaignGenomeMatches,
		OperationalMemory: operationalMemory, FundingOutcomeMemory: fundingOutcomeMemory,
		IncidentCorpus: incidentCorpus, ActorIncidentHistory: actorIncidentHistory,
		BehavioralSignatures: behavioralSignatures, Behavior: behavior, UnifiedVerdict: unifiedVerdict,
		Threat: threat, CombinedEvidence: combinedEvidence, Modules: modules, Structural: structural,
		Graph: graph, TradeLedger: tradeLedger, ActorStoreStatus: actorStoreStatus,
	}
}
