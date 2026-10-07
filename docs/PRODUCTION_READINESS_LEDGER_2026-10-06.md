# Koschei Web3 Hub — Full Production Readiness Ledger

**Date:** 2026-10-06  
**Audit source:** `repo-denetim-ham-listeler.md` captured at `main@9a19ed7`  
**Current comparison base:** `main@6b1fb62872747482bb9b8456396b130e3aa01db7`  
**Policy:** NO DELETE. NO SKIP. Every finding stays tracked until it is either wired to production, explicitly retained as intentional test/compat code, or blocked from public HTTP while preserved in the repository.

## Current delta from audit snapshot

- The repository is 18 commits ahead of the audit snapshot.
- Only 9 files changed/appeared between the audit snapshot and current main: owner auth/UI files plus the ARVIS stage-isolated scan runtime and one holder-core call site.
- None of the source files listed in sections A or B were directly modified by that delta.
- New stage-isolation code does not intentionally retire the old work; all A/B findings remain in this ledger until reachability is re-proven by CI/deadcode after production wiring.

## Production acceptance gates

- [ ] Owner login and authenticated owner shell are production-safe and mobile-usable.
- [ ] Customer panel/auth/Professional entitlement flow is end-to-end accepted.
- [ ] ARVIS full scan executes all applicable collectors independently; one RPC failure cannot poison later collectors.
- [ ] Primary + independent fallback RPC are healthy under bounded rate governance.
- [ ] Background Radar workers are active under bounded production budgets, not silently paused.
- [ ] ARVIS results are delivered through canonical UI/report/API and opted-in Telegram.
- [ ] Public static surface does not expose test fixtures/tests/internal-only assets.
- [ ] No frontend references an unregistered live API route.
- [ ] All CI/release/security/public-smoke gates are green with current owner-auth contract.

## A — Completely unreachable functions (215)

| # | Finding | Production disposition |
|---:|---|---|
| 1 | `internal/agents/admin.go:317:19: unreachable func: Service.AdminUpdateAppointment` | **P2 INTENT REVIEW** |
| 2 | `internal/agents/channel_delivery.go:23:6: unreachable func: WhatsAppOutboundEnabled` | **P2 INTENT REVIEW** |
| 3 | `internal/agents/channel_delivery.go:31:6: unreachable func: SendWhatsAppText` | **P2 INTENT REVIEW** |
| 4 | `internal/agents/llm.go:40:21: unreachable func: LLMClient.Rewrite` | **P2 INTENT REVIEW** |
| 5 | `internal/cache/compat.go:3:6: unreachable func: NewMemoryCache` | **P2 INTENT REVIEW** |
| 6 | `internal/cryptobrief/http.go:226:19: unreachable func: Service.WhatsAppHTTP` | **P2 INTENT REVIEW** |
| 7 | `internal/cryptobrief/store.go:483:6: unreachable func: maxTime` | **P2 INTENT REVIEW** |
| 8 | `internal/defense/authority_binding_v01_test.go:41:6: unreachable func: authorityIntegrityScenarioHash` | **P2 INTENT REVIEW** |
| 9 | `internal/defense/control_contracts.go:192:6: unreachable func: SortedEvidenceIDs` | **P2 INTENT REVIEW** |
| 10 | `internal/defense/harness.go:345:6: unreachable func: AttestLocalToolchain` | **P2 INTENT REVIEW** |
| 11 | `internal/defense/harness.go:405:6: unreachable func: ListToolchainAttestations` | **P2 INTENT REVIEW** |
| 12 | `internal/defense/safe_execution.go:53:6: unreachable func: CreateToolchainPolicy` | **P2 INTENT REVIEW** |
| 13 | `internal/executionproof/anvil_backend_v03.go:36:27: unreachable func: AnvilForkBackend.ExecuteVerifiedFork` | **P2 INTENT REVIEW** |
| 14 | `internal/executionproof/anvil_backend_v03.go:269:24: unreachable func: evmRPCClient.callBool` | **P2 INTENT REVIEW** |
| 15 | `internal/executionproof/anvil_backend_v03.go:280:24: unreachable func: evmRPCClient.sendTransaction` | **P2 INTENT REVIEW** |
| 16 | `internal/executionproof/safe_isolated_runner.go:232:6: unreachable func: safeOutflowsWithinPolicy` | **P2 INTENT REVIEW** |
| 17 | `internal/executionproof/safe_trace_verifier.go:78:6: unreachable func: canonicalizeTraceFrames` | **P2 INTENT REVIEW** |
| 18 | `internal/handlers/actor_acceptance_liquidity.go:228:6: unreachable func: actorAcceptanceSortedMints` | **P0/P1 PROD WIRING REVIEW** |
| 19 | `internal/handlers/ai.go:11:6: unreachable func: AIRunHandler` | **P2 INTENT REVIEW** |
| 20 | `internal/handlers/api_errors.go:33:6: unreachable func: writeAPISuccess` | **P2 INTENT REVIEW** |
| 21 | `internal/handlers/arvis_investigation_projection_attach.go:23:6: unreachable func: attachCanonicalArvisInvestigationsToReport` | **P0/P1 PROD WIRING REVIEW** |
| 22 | `internal/handlers/arvis_sections.go:5:6: unreachable func: arvisSections` | **P0/P1 PROD WIRING REVIEW** |
| 23 | `internal/handlers/autopublish_policy.go:60:6: unreachable func: defaultAutopublishThresholds` | **P0/P1 PROD WIRING REVIEW** |
| 24 | `internal/handlers/autopublish_policy.go:215:33: unreachable func: autopublishThresholds.asMap` | **P0/P1 PROD WIRING REVIEW** |
| 25 | `internal/handlers/autopublish_worker.go:41:6: unreachable func: StartAutopublishWorker` | **P0/P1 PROD WIRING REVIEW** |
| 26 | `internal/handlers/autopublish_worker.go:62:29: unreachable func: autopublishWorker.Start` | **P0/P1 PROD WIRING REVIEW** |
| 27 | `internal/handlers/autopublish_worker.go:99:29: unreachable func: autopublishWorker.RunOnce` | **P0/P1 PROD WIRING REVIEW** |
| 28 | `internal/handlers/autopublish_worker.go:121:29: unreachable func: autopublishWorker.loadCandidates` | **P0/P1 PROD WIRING REVIEW** |
| 29 | `internal/handlers/autopublish_worker.go:148:29: unreachable func: autopublishWorker.decide` | **P0/P1 PROD WIRING REVIEW** |
| 30 | `internal/handlers/autopublish_worker.go:171:29: unreachable func: autopublishWorker.record` | **P0/P1 PROD WIRING REVIEW** |
| 31 | `internal/handlers/autopublish_worker.go:263:6: unreachable func: verifyAutopublishPublicationBundle` | **P0/P1 PROD WIRING REVIEW** |
| 32 | `internal/handlers/autopublish_worker.go:283:29: unreachable func: autopublishWorker.now` | **P0/P1 PROD WIRING REVIEW** |
| 33 | `internal/handlers/canonical_pump_job_scheduler.go:209:6: unreachable func: canonicalPumpJobDedupeKey` | **P0/P1 PROD WIRING REVIEW** |
| 34 | `internal/handlers/compat.go:201:19: unreachable func: Handler.callTogetherWithSystemTimeoutAndMaxTokens` | **P2 INTENT REVIEW** |
| 35 | `internal/handlers/court_narrative.go:184:19: unreachable func: Handler.courtScheduledReport` | **P2 INTENT REVIEW** |
| 36 | `internal/handlers/credits_atomic.go:135:19: unreachable func: Handler.hasActivePaidPackage` | **P2 INTENT REVIEW** |
| 37 | `internal/handlers/dossier_rows.go:78:6: unreachable func: dossierFindModule` | **P0/P1 PROD WIRING REVIEW** |
| 38 | `internal/handlers/dossier_rows.go:83:6: unreachable func: dossierFindBehavior` | **P0/P1 PROD WIRING REVIEW** |
| 39 | `internal/handlers/handlers_common.go:85:6: unreachable func: isTransientDBError` | **P2 INTENT REVIEW** |
| 40 | `internal/handlers/handlers_common.go:114:19: unreachable func: Handler.requireAdmin` | **P2 INTENT REVIEW** |
| 41 | `internal/handlers/impact_metrics.go:42:6: unreachable func: protectedImpactMetrics` | **P2 INTENT REVIEW** |
| 42 | `internal/handlers/impact_metrics.go:74:6: unreachable func: addProtectedSeries` | **P2 INTENT REVIEW** |
| 43 | `internal/handlers/impact_metrics.go:96:6: unreachable func: ensureDailyImpactTweetDraft` | **P2 INTENT REVIEW** |
| 44 | `internal/handlers/impact_metrics.go:113:6: unreachable func: draftForImpact` | **P2 INTENT REVIEW** |
| 45 | `internal/handlers/impact_metrics.go:119:6: unreachable func: roundMoney` | **P2 INTENT REVIEW** |
| 46 | `internal/handlers/intelligence_os.go:70:6: unreachable func: grantContent` | **P2 INTENT REVIEW** |
| 47 | `internal/handlers/jwt_verify.go:12:6: unreachable func: decodeBase64Raw` | **P2 INTENT REVIEW** |
| 48 | `internal/handlers/jwt_verify.go:16:6: unreachable func: decodeBase64JSON` | **P2 INTENT REVIEW** |
| 49 | `internal/handlers/jwt_verify.go:24:6: unreachable func: verifyJWTSignatureRS256` | **P2 INTENT REVIEW** |
| 50 | `internal/handlers/jwt_verify.go:34:6: unreachable func: splitToken` | **P2 INTENT REVIEW** |
| 51 | `internal/handlers/legacy_token_audit_compat.go:17:6: unreachable func: configuredKoscheiTokenGateEnabled` | **P2 INTENT REVIEW** |
| 52 | `internal/handlers/legacy_token_audit_compat.go:21:6: unreachable func: tokenTierThresholdEnv` | **P2 INTENT REVIEW** |
| 53 | `internal/handlers/limiter_compat.go:3:6: unreachable func: NewRateLimiter` | **P2 INTENT REVIEW** |
| 54 | `internal/handlers/lp_market_context.go:51:19: unreachable func: Handler.collectLPControlEvidence` | **P0/P1 PROD WIRING REVIEW** |
| 55 | `internal/handlers/lp_market_context.go:268:19: unreachable func: Handler.collectJupiterMarketContext` | **P0/P1 PROD WIRING REVIEW** |
| 56 | `internal/handlers/mev_shield.go:63:6: unreachable func: AnalyzeMEV` | **P0/P1 PROD WIRING REVIEW** |
| 57 | `internal/handlers/neon_auth_config.go:121:6: unreachable func: ConfiguredPublicNeonAuthURL` | **P2 INTENT REVIEW** |
| 58 | `internal/handlers/neon_auth_config.go:125:6: unreachable func: ConfiguredNeonAuthIssuer` | **P2 INTENT REVIEW** |
| 59 | `internal/handlers/neon_auth_only_mode.go:8:6: unreachable func: neonAuthOnlyMode` | **P2 INTENT REVIEW** |
| 60 | `internal/handlers/owner_command_center.go:265:6: unreachable func: dbCount` | **P2 INTENT REVIEW** |
| 61 | `internal/handlers/owner_command_center.go:280:6: unreachable func: pumpPortalStatus` | **P0/P1 PROD WIRING REVIEW** |
| 62 | `internal/handlers/owner_command_center.go:290:6: unreachable func: radarWorkerStatus` | **P0/P1 PROD WIRING REVIEW** |
| 63 | `internal/handlers/owner_command_center.go:297:6: unreachable func: truthyOwnerEnv` | **P2 INTENT REVIEW** |
| 64 | `internal/handlers/owner_operations.go:341:6: unreachable func: ownerRadarModuleSignal` | **P0/P1 PROD WIRING REVIEW** |
| 65 | `internal/handlers/owner_operations.go:376:6: unreachable func: ownerRadarPendingNarrative` | **P0/P1 PROD WIRING REVIEW** |
| 66 | `internal/handlers/owner_operations.go:438:6: unreachable func: ownerRadarStringSlice` | **P0/P1 PROD WIRING REVIEW** |
| 67 | `internal/handlers/owner_operations.go:465:6: unreachable func: ownerRadarRiskLabelTR` | **P0/P1 PROD WIRING REVIEW** |
| 68 | `internal/handlers/owner_operations.go:480:6: unreachable func: ownerRadarRiskMeaning` | **P0/P1 PROD WIRING REVIEW** |
| 69 | `internal/handlers/owner_operations.go:493:6: unreachable func: ownerRadarRoleTR` | **P0/P1 PROD WIRING REVIEW** |
| 70 | `internal/handlers/owner_operations.go:513:6: unreachable func: ownerRadarHolderMeaning` | **P0/P1 PROD WIRING REVIEW** |
| 71 | `internal/handlers/owner_operations.go:528:6: unreachable func: ownerRadarPracticalConclusion` | **P0/P1 PROD WIRING REVIEW** |
| 72 | `internal/handlers/owner_payment_health.go:101:6: unreachable func: writePaymentAudit` | **P2 INTENT REVIEW** |
| 73 | `internal/handlers/owner_payment_health.go:108:6: unreachable func: sanitizePaymentAuditMetadata` | **P2 INTENT REVIEW** |
| 74 | `internal/handlers/owner_unified_radar.go:412:6: unreachable func: ownerCourtUnavailableReport` | **P0/P1 PROD WIRING REVIEW** |
| 75 | `internal/handlers/payments.go:50:6: unreachable func: packageName` | **P2 INTENT REVIEW** |
| 76 | `internal/handlers/platform.go:45:6: unreachable func: configuredURL` | **P2 INTENT REVIEW** |
| 77 | `internal/handlers/security_ecosystem_sections.go:20:6: unreachable func: securityEcosystemSections` | **P0/P1 PROD WIRING REVIEW** |
| 78 | `internal/handlers/security_ecosystem_sections.go:133:6: unreachable func: deterministicRiskScore` | **P2 INTENT REVIEW** |
| 79 | `internal/handlers/security_ecosystem_sections.go:146:6: unreachable func: riskLevelFromScore` | **P2 INTENT REVIEW** |
| 80 | `internal/handlers/security_ecosystem_sections.go:159:6: unreachable func: shortTarget` | **P2 INTENT REVIEW** |
| 81 | `internal/handlers/security_ecosystem_sections.go:167:6: unreachable func: clampSecurityRisk` | **P2 INTENT REVIEW** |
| 82 | `internal/handlers/security_ecosystem_sections.go:177:6: unreachable func: recommendationFromRisk` | **P2 INTENT REVIEW** |
| 83 | `internal/handlers/security_ecosystem_sections.go:190:6: unreachable func: verdictFromRisk` | **P2 INTENT REVIEW** |
| 84 | `internal/handlers/security_ecosystem_sections.go:203:6: unreachable func: signaturePreview` | **P2 INTENT REVIEW** |
| 85 | `internal/handlers/security_radar.go:674:6: unreachable func: jsonRaw` | **P0/P1 PROD WIRING REVIEW** |
| 86 | `internal/handlers/security_radar_exposure_token2022.go:76:6: unreachable func: exposureToken2022Status` | **P0/P1 PROD WIRING REVIEW** |
| 87 | `internal/handlers/shield.go:112:6: unreachable func: shieldReason` | **P2 INTENT REVIEW** |
| 88 | `internal/handlers/transaction_guard_enforcement_permit.go:296:6: unreachable func: transactionGuardPermitPublicKeyFingerprint` | **P0/P1 PROD WIRING REVIEW** |
| 89 | `internal/handlers/transaction_guard_v2.go:622:6: unreachable func: guardSlippagePercent` | **P0/P1 PROD WIRING REVIEW** |
| 90 | `internal/handlers/transaction_guard_v2_evidence_first.go:173:19: unreachable func: Handler.finishUnavailableTransactionGuardV2` | **P0/P1 PROD WIRING REVIEW** |
| 91 | `internal/handlers/transaction_guard_v3_authority_integration.go:32:6: unreachable func: removeTransactionGuardV3LegacyAuthorityFindings` | **P0/P1 PROD WIRING REVIEW** |
| 92 | `internal/handlers/transaction_guard_v3_models.go:174:6: unreachable func: guardV3AddressForIndex` | **P0/P1 PROD WIRING REVIEW** |
| 93 | `internal/handlers/transaction_guard_v3_response.go:76:19: unreachable func: Handler.finishTransactionGuardV3Response` | **P0/P1 PROD WIRING REVIEW** |
| 94 | `internal/handlers/unified_wallet_investigation_report.go:235:6: unreachable func: canonicalWalletDB` | **P0/P1 PROD WIRING REVIEW** |
| 95 | `internal/handlers/validation.go:15:6: unreachable func: validPlan` | **P2 INTENT REVIEW** |
| 96 | `internal/handlers/validation.go:24:6: unreachable func: validStatus` | **P2 INTENT REVIEW** |
| 97 | `internal/handlers/validation.go:66:6: unreachable func: validPaidActivationPlan` | **P2 INTENT REVIEW** |
| 98 | `internal/handlers/watchlist_monitor.go:24:6: unreachable func: StartWatchlistMonitor` | **P0/P1 PROD WIRING REVIEW** |
| 99 | `internal/handlers/watchlist_monitor.go:49:6: unreachable func: runWatchlistMonitorBatch` | **P0/P1 PROD WIRING REVIEW** |
| 100 | `internal/handlers/watchlist_monitor.go:79:6: unreachable func: claimDueWatchlistTargets` | **P0/P1 PROD WIRING REVIEW** |
| 101 | `internal/handlers/web3.go:776:6: unreachable func: chainFromNetwork` | **P0/P1 PROD WIRING REVIEW** |
| 102 | `internal/handlers/web3.go:790:6: unreachable func: normalizeAddressForStorage` | **P0/P1 PROD WIRING REVIEW** |
| 103 | `internal/handlers/web3.go:798:6: unreachable func: normalizeNetwork` | **P0/P1 PROD WIRING REVIEW** |
| 104 | `internal/handlers/web3.go:802:6: unreachable func: displayNetwork` | **P0/P1 PROD WIRING REVIEW** |
| 105 | `internal/handlers/web3.go:821:6: unreachable func: parseLimit` | **P0/P1 PROD WIRING REVIEW** |
| 106 | `internal/handlers/web3.go:832:6: unreachable func: findValueByKeys` | **P0/P1 PROD WIRING REVIEW** |
| 107 | `internal/handlers/web3.go:852:6: unreachable func: findStringByKeys` | **P0/P1 PROD WIRING REVIEW** |
| 108 | `internal/handlers/web3.go:856:6: unreachable func: findNestedString` | **P0/P1 PROD WIRING REVIEW** |
| 109 | `internal/handlers/web3.go:877:6: unreachable func: stringifyWeb3JSONValue` | **P0/P1 PROD WIRING REVIEW** |
| 110 | `internal/handlers/web3.go:901:6: unreachable func: uniqueNonEmptyStrings` | **P0/P1 PROD WIRING REVIEW** |
| 111 | `internal/handlers/webhooks.go:451:6: unreachable func: enqueueWatchlistWebhookDeliveries` | **P0/P1 PROD WIRING REVIEW** |
| 112 | `internal/http/customer_scan_routes.go:75:6: unreachable func: customerScan` | **P2 INTENT REVIEW** |
| 113 | `internal/http/server_helpers.go:108:6: unreachable func: registerLegacyDashboardRedirects` | **P2 INTENT REVIEW** |
| 114 | `internal/http/server_helpers.go:116:6: unreachable func: redirectToDashboard` | **P2 INTENT REVIEW** |
| 115 | `internal/mocks/mock_ai_client.go:12:23: unreachable func: MockAIClient.Analyze` | **P2 INTENT REVIEW** |
| 116 | `internal/mocks/mock_rpc_client.go:17:6: unreachable func: NewMockRPCClient` | **P2 INTENT REVIEW** |
| 117 | `internal/mocks/mock_rpc_client.go:21:25: unreachable func: MockRPCClient.Call` | **P2 INTENT REVIEW** |
| 118 | `internal/networktarget/bitcoin_core_node_telemetry.go:141:6: unreachable func: bitcoinCoreRPC` | **P0/P1 PROD WIRING REVIEW** |
| 119 | `internal/networktarget/bitcoin_probe.go:138:6: unreachable func: bitcoinEsploraText` | **P0/P1 PROD WIRING REVIEW** |
| 120 | `internal/networktarget/bitcoin_probe.go:172:6: unreachable func: bitcoinEsploraJSON` | **P0/P1 PROD WIRING REVIEW** |
| 121 | `internal/networktarget/ethereum_beacon_telemetry.go:210:6: unreachable func: ethereumBeaconGETJSON` | **P0/P1 PROD WIRING REVIEW** |
| 122 | `internal/networktarget/evm_node_telemetry.go:147:6: unreachable func: evmNodeTelemetryRPCRaw` | **P0/P1 PROD WIRING REVIEW** |
| 123 | `internal/networktarget/move_identity_probe.go:224:6: unreachable func: moveIdentityDoJSON` | **P0/P1 PROD WIRING REVIEW** |
| 124 | `internal/radarevent/head_observation_adapter.go:11:6: unreachable func: BuildEVMHeadBlockEvent` | **P0/P1 PROD WIRING REVIEW** |
| 125 | `internal/radarevent/head_observation_adapter.go:36:6: unreachable func: BuildBitcoinHeadBlockEvent` | **P0/P1 PROD WIRING REVIEW** |
| 126 | `internal/runtimecfg/config.go:98:6: unreachable func: AIProviderAvailable` | **P2 INTENT REVIEW** |
| 127 | `internal/services/actor_defense_correlator.go:25:6: unreachable func: NewActorDefenseCorrelator` | **P0/P1 PROD WIRING REVIEW** |
| 128 | `internal/services/actor_defense_correlator.go:29:34: unreachable func: ActorDefenseCorrelator.Start` | **P0/P1 PROD WIRING REVIEW** |
| 129 | `internal/services/actor_defense_correlator.go:61:34: unreachable func: ActorDefenseCorrelator.RunOnce` | **P0/P1 PROD WIRING REVIEW** |
| 130 | `internal/services/actor_defense_start.go:9:6: unreachable func: StartActorDefenseCorrelator` | **P0/P1 PROD WIRING REVIEW** |
| 131 | `internal/services/actor_initial_recipient_transport.go:336:6: unreachable func: validateActorInitialRecipientTransport` | **P0/P1 PROD WIRING REVIEW** |
| 132 | `internal/services/arvis_transaction_arms.go:41:6: unreachable func: AnalyzeArvisRadarsWithTransactions` | **P0/P1 PROD WIRING REVIEW** |
| 133 | `internal/services/arvis_transaction_arms.go:89:6: unreachable func: collectArvisTransactionEvidence` | **P0/P1 PROD WIRING REVIEW** |
| 134 | `internal/services/arvis_transaction_arms.go:617:6: unreachable func: verifiedArvisArmCount` | **P0/P1 PROD WIRING REVIEW** |
| 135 | `internal/services/arvis_transaction_bundle.go:9:6: unreachable func: EnrichArvisBundleWithTransactions` | **P0/P1 PROD WIRING REVIEW** |
| 136 | `internal/services/helius_asset_metadata.go:182:6: unreachable func: resetHeliusAssetMetadataCacheForTest` | **P0/P1 PROD WIRING REVIEW** |
| 137 | `internal/services/holder_cluster_intelligence.go:176:6: unreachable func: analyzeHolderClusterWallet` | **P0/P1 PROD WIRING REVIEW** |
| 138 | `internal/services/holder_flow_entity_classification.go:313:6: unreachable func: sortedHolderFlowKeys` | **P0/P1 PROD WIRING REVIEW** |
| 139 | `internal/services/provider_witness_memory.go:316:6: unreachable func: ValidateProviderWitnessMemory` | **P2 INTENT REVIEW** |
| 140 | `internal/services/pump_high_volume_radar.go:260:6: unreachable func: pumpHighVolumeReportCooldown` | **P0/P1 PROD WIRING REVIEW** |
| 141 | `internal/services/pump_high_volume_radar.go:278:6: unreachable func: pumpHighVolumePageSize` | **P0/P1 PROD WIRING REVIEW** |
| 142 | `internal/services/pump_high_volume_radar.go:314:6: unreachable func: NewPumpHighVolumeRadarWorker` | **P0/P1 PROD WIRING REVIEW** |
| 143 | `internal/services/pump_high_volume_radar.go:326:37: unreachable func: PumpHighVolumeRadarWorker.Start` | **P0/P1 PROD WIRING REVIEW** |
| 144 | `internal/services/pump_high_volume_radar.go:348:37: unreachable func: PumpHighVolumeRadarWorker.RunOnce` | **P0/P1 PROD WIRING REVIEW** |
| 145 | `internal/services/pump_high_volume_radar.go:456:37: unreachable func: PumpHighVolumeRadarWorker.scanAndStore` | **P0/P1 PROD WIRING REVIEW** |
| 146 | `internal/services/pump_trade_ledger.go:38:6: unreachable func: NewPumpTradeLedgerWriter` | **P0/P1 PROD WIRING REVIEW** |
| 147 | `internal/services/pump_trade_ledger.go:46:33: unreachable func: PumpTradeLedgerWriter.PersistPumpPortal` | **P0/P1 PROD WIRING REVIEW** |
| 148 | `internal/services/pump_trade_ledger.go:95:33: unreachable func: PumpTradeLedgerWriter.Start` | **P0/P1 PROD WIRING REVIEW** |
| 149 | `internal/services/pump_trade_ledger.go:139:33: unreachable func: PumpTradeLedgerWriter.insertBatch` | **P0/P1 PROD WIRING REVIEW** |
| 150 | `internal/services/pumpportal_client.go:53:28: unreachable func: PumpPortalClient.Start` | **P0/P1 PROD WIRING REVIEW** |
| 151 | `internal/services/pumpportal_client.go:84:28: unreachable func: PumpPortalClient.run` | **P0/P1 PROD WIRING REVIEW** |
| 152 | `internal/services/pumpportal_client.go:178:28: unreachable func: PumpPortalClient.shouldTrackMint` | **P0/P1 PROD WIRING REVIEW** |
| 153 | `internal/services/pumpportal_client.go:209:28: unreachable func: PumpPortalClient.tradeSubscriptionBatches` | **P0/P1 PROD WIRING REVIEW** |
| 154 | `internal/services/pumpportal_client.go:233:6: unreachable func: isPumpPortalTradeEvent` | **P0/P1 PROD WIRING REVIEW** |
| 155 | `internal/services/pumpportal_client.go:341:6: unreachable func: dialPumpPortalWebSocket` | **P0/P1 PROD WIRING REVIEW** |
| 156 | `internal/services/pumpportal_client.go:409:24: unreachable func: bufferedConn.Read` | **P0/P1 PROD WIRING REVIEW** |
| 157 | `internal/services/pumpportal_client.go:411:6: unreachable func: websocketAcceptKey` | **P0/P1 PROD WIRING REVIEW** |
| 158 | `internal/services/pumpportal_client.go:416:6: unreachable func: writeWebSocketText` | **P0/P1 PROD WIRING REVIEW** |
| 159 | `internal/services/pumpportal_client.go:424:6: unreachable func: writeWebSocketClose` | **P0/P1 PROD WIRING REVIEW** |
| 160 | `internal/services/pumpportal_client.go:426:6: unreachable func: writeWebSocketControl` | **P0/P1 PROD WIRING REVIEW** |
| 161 | `internal/services/pumpportal_client.go:430:6: unreachable func: writeWebSocketFrame` | **P0/P1 PROD WIRING REVIEW** |
| 162 | `internal/services/pumpportal_client.go:463:6: unreachable func: readWebSocketFrame` | **P0/P1 PROD WIRING REVIEW** |
| 163 | `internal/services/pumpportal_config.go:29:27: unreachable func: PumpPortalConfig.websocketURL` | **P0/P1 PROD WIRING REVIEW** |
| 164 | `internal/services/pumpportal_config.go:47:27: unreachable func: PumpPortalConfig.redactedWebsocketHost` | **P0/P1 PROD WIRING REVIEW** |
| 165 | `internal/services/pumpportal_durable_inbox.go:34:6: unreachable func: NewPumpPortalDurableInbox` | **P0/P1 PROD WIRING REVIEW** |
| 166 | `internal/services/pumpportal_durable_inbox.go:42:34: unreachable func: PumpPortalDurableInbox.PersistDiscovery` | **P0/P1 PROD WIRING REVIEW** |
| 167 | `internal/services/pumpportal_durable_inbox.go:94:34: unreachable func: PumpPortalDurableInbox.Start` | **P0/P1 PROD WIRING REVIEW** |
| 168 | `internal/services/pumpportal_durable_inbox.go:114:34: unreachable func: PumpPortalDurableInbox.signal` | **P0/P1 PROD WIRING REVIEW** |
| 169 | `internal/services/pumpportal_durable_inbox.go:124:34: unreachable func: PumpPortalDurableInbox.processAvailable` | **P0/P1 PROD WIRING REVIEW** |
| 170 | `internal/services/pumpportal_durable_inbox.go:145:34: unreachable func: PumpPortalDurableInbox.claimBatch` | **P0/P1 PROD WIRING REVIEW** |
| 171 | `internal/services/pumpportal_durable_inbox.go:182:34: unreachable func: PumpPortalDurableInbox.processOne` | **P0/P1 PROD WIRING REVIEW** |
| 172 | `internal/services/pumpportal_durable_inbox.go:192:34: unreachable func: PumpPortalDurableInbox.finish` | **P0/P1 PROD WIRING REVIEW** |
| 173 | `internal/services/pumpportal_inbox_health.go:98:6: unreachable func: pumpPortalInboxHealthIsHealthy` | **P0/P1 PROD WIRING REVIEW** |
| 174 | `internal/services/pumpportal_observable_client.go:26:6: unreachable func: setPumpPortalTradeRuntime` | **P0/P1 PROD WIRING REVIEW** |
| 175 | `internal/services/pumpportal_observable_client.go:48:6: unreachable func: NewPumpPortalObservableClient` | **P0/P1 PROD WIRING REVIEW** |
| 176 | `internal/services/pumpportal_observable_client.go:58:38: unreachable func: PumpPortalObservableClient.Start` | **P0/P1 PROD WIRING REVIEW** |
| 177 | `internal/services/pumpportal_observable_client.go:86:38: unreachable func: PumpPortalObservableClient.run` | **P0/P1 PROD WIRING REVIEW** |
| 178 | `internal/services/pumpportal_radar_adapter.go:18:6: unreachable func: NewPumpPortalRadarAdapter` | **P0/P1 PROD WIRING REVIEW** |
| 179 | `internal/services/pumpportal_radar_adapter.go:22:34: unreachable func: PumpPortalRadarAdapter.Start` | **P0/P1 PROD WIRING REVIEW** |
| 180 | `internal/services/pumpportal_radar_adapter.go:41:34: unreachable func: PumpPortalRadarAdapter.Enqueue` | **P0/P1 PROD WIRING REVIEW** |
| 181 | `internal/services/pumpportal_radar_adapter.go:54:34: unreachable func: PumpPortalRadarAdapter.HandleEvent` | **P0/P1 PROD WIRING REVIEW** |
| 182 | `internal/services/pumpportal_radar_adapter.go:110:6: unreachable func: pumpPortalSignals` | **P0/P1 PROD WIRING REVIEW** |
| 183 | `internal/services/pumpportal_radar_adapter.go:140:6: unreachable func: pumpPortalArmVerified` | **P0/P1 PROD WIRING REVIEW** |
| 184 | `internal/services/pumpportal_radar_adapter.go:144:6: unreachable func: pumpPortalWarningLabel` | **P0/P1 PROD WIRING REVIEW** |
| 185 | `internal/services/pumpportal_radar_adapter.go:155:6: unreachable func: StartPumpPortalRadarIfEnabled` | **P0/P1 PROD WIRING REVIEW** |
| 186 | `internal/services/pumpportal_radar_adapter.go:193:6: unreachable func: canonicalInvestigationWorkerActive` | **P0/P1 PROD WIRING REVIEW** |
| 187 | `internal/services/pumpportal_radar_adapter.go:226:6: unreachable func: mergePumpPortalSignals` | **P0/P1 PROD WIRING REVIEW** |
| 188 | `internal/services/secret_safety.go:5:6: unreachable func: RedactSecret` | **P2 INTENT REVIEW** |
| 189 | `internal/services/secret_safety.go:16:6: unreachable func: SafeLogKey` | **P2 INTENT REVIEW** |
| 190 | `internal/services/security_incident_corpus.go:101:6: unreachable func: MaterializeVerifiedIncidentCorpus` | **P0/P1 PROD WIRING REVIEW** |
| 191 | `internal/services/security_radar_retention_worker.go:90:6: unreachable func: StartSecurityRadarRetentionWorker` | **P0/P1 PROD WIRING REVIEW** |
| 192 | `internal/services/security_radar_retention_worker.go:120:40: unreachable func: securityRadarRetentionWorker.start` | **P0/P1 PROD WIRING REVIEW** |
| 193 | `internal/services/security_radar_retention_worker.go:139:40: unreachable func: securityRadarRetentionWorker.runOnce` | **P0/P1 PROD WIRING REVIEW** |
| 194 | `internal/services/security_radar_retention_worker.go:251:40: unreachable func: securityRadarRetentionWorker.archiveAndDelete` | **P0/P1 PROD WIRING REVIEW** |
| 195 | `internal/services/security_radar_stream_worker.go:566:6: unreachable func: extractRadarMintFromLogs` | **P0/P1 PROD WIRING REVIEW** |
| 196 | `internal/services/security_radars.go:113:6: unreachable func: AnalyzeSecurityRadars` | **P0/P1 PROD WIRING REVIEW** |
| 197 | `internal/services/security_radars.go:192:6: unreachable func: collectRadarEvidence` | **P0/P1 PROD WIRING REVIEW** |
| 198 | `internal/services/security_radars.go:316:6: unreachable func: buildPumpSybilVerdict` | **P0/P1 PROD WIRING REVIEW** |
| 199 | `internal/services/security_radars.go:352:6: unreachable func: buildRaydiumPoolVerdict` | **P0/P1 PROD WIRING REVIEW** |
| 200 | `internal/services/security_radars.go:393:6: unreachable func: buildClaimShieldVerdict` | **P0/P1 PROD WIRING REVIEW** |
| 201 | `internal/services/security_radars.go:401:6: unreachable func: newRadarVerdict` | **P0/P1 PROD WIRING REVIEW** |
| 202 | `internal/services/security_radars.go:424:6: unreachable func: baseEvidenceSignals` | **P0/P1 PROD WIRING REVIEW** |
| 203 | `internal/services/security_radars.go:506:6: unreachable func: burstRisk` | **P0/P1 PROD WIRING REVIEW** |
| 204 | `internal/services/solana_rpc_batch.go:369:6: unreachable func: resetSolanaRPCBatchModeCacheForTest` | **P0/P1 PROD WIRING REVIEW** |
| 205 | `internal/services/unified_engine.go:692:31: unreachable func: UnifiedAnalyzeResult.ModuleResultsJSON` | **P2 INTENT REVIEW** |
| 206 | `internal/web3/evidence_court_canonical.go:270:6: unreachable func: validateEvidenceCourtCanonicalHash` | **P0/P1 PROD WIRING REVIEW** |
| 207 | `pkg/agent/core.go:27:6: unreachable func: CallAI` | **P2 INTENT REVIEW** |
| 208 | `pkg/agent/core.go:50:6: unreachable func: buildSystemPrompt` | **P2 INTENT REVIEW** |
| 209 | `pkg/agent/git.go:8:6: unreachable func: RunGitCommand` | **P2 INTENT REVIEW** |
| 210 | `pkg/agent/git.go:18:6: unreachable func: CreateBranch` | **P2 INTENT REVIEW** |
| 211 | `pkg/agent/git.go:22:6: unreachable func: AddAndCommit` | **P2 INTENT REVIEW** |
| 212 | `pkg/agent/git.go:27:6: unreachable func: PushBranch` | **P2 INTENT REVIEW** |
| 213 | `pkg/agent/github.go:23:6: unreachable func: CreatePullRequest` | **P2 INTENT REVIEW** |
| 214 | `pkg/audit/logger.go:12:6: unreachable func: Init` | **P2 INTENT REVIEW** |
| 215 | `pkg/audit/logger.go:26:6: unreachable func: Log` | **P2 INTENT REVIEW** |

## B — Test-only reachable functions (353)

| # | Finding | Production disposition |
|---:|---|---|
| 1 | `internal/clickhouse/publication_ledger.go: unreachable func: BuildPublicationTransition` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 2 | `internal/clickhouse/publication_ledger.go: unreachable func: LatestPublicationState` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 3 | `internal/clickhouse/publication_ledger.go: unreachable func: NormalizeDossierBundleManifest` | **P0/P1 TEST→PROD REVIEW** |
| 4 | `internal/clickhouse/publication_ledger.go: unreachable func: ValidatePublicationTransitionChain` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 5 | `internal/clickhouse/publication_ledger.go: unreachable func: deterministicUUID` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 6 | `internal/clickhouse/publication_ledger.go: unreachable func: dossierManifestSHA256` | **P0/P1 TEST→PROD REVIEW** |
| 7 | `internal/clickhouse/publication_ledger.go: unreachable func: formatUUID` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 8 | `internal/clickhouse/publication_ledger.go: unreachable func: hashFields` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 9 | `internal/clickhouse/publication_ledger.go: unreachable func: normalizeHex64` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 10 | `internal/clickhouse/publication_ledger.go: unreachable func: publicationAction` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 11 | `internal/clickhouse/publication_ledger.go: unreachable func: publicationActor` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 12 | `internal/clickhouse/publication_ledger.go: unreachable func: publicationTransitionCanonical` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 13 | `internal/clickhouse/publication_ledger.go: unreachable func: publicationTransitionID` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 14 | `internal/clickhouse/publication_ledger.go: unreachable func: publicationTransitionSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 15 | `internal/clickhouse/publication_ledger.go: unreachable func: sameOptionalMillisecondTime` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 16 | `internal/clickhouse/publication_ledger.go: unreachable func: uuidFromDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 17 | `internal/clickhouse/publication_ledger.go: unreachable func: validateEvidenceArtifactURI` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 18 | `internal/clickhouse/publication_ledger.go: unreachable func: validatePublicationTransitionSelf` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 19 | `internal/cryptobrief/feed.go: unreachable func: Matches` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 20 | `internal/cryptobrief/http.go: unreachable func: VerifyWhatsAppSignature` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 21 | `internal/decision/decision.go: unreachable func: FromExecutionContainment` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 22 | `internal/decision/decision.go: unreachable func: FromTransactionGuard` | **P0/P1 TEST→PROD REVIEW** |
| 23 | `internal/decision/decision.go: unreachable func: NormalizeAction` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 24 | `internal/defense/authority_binding_v01.go: unreachable func: AdaptAuthorityIntegrityCaseV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 25 | `internal/defense/authority_binding_v01.go: unreachable func: AdaptAuthoritySecurityEvidenceObservationV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 26 | `internal/defense/authority_binding_v01.go: unreachable func: ApplyDefenseAuthorityBindingToContainmentV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 27 | `internal/defense/authority_binding_v01.go: unreachable func: DefenseAuthorityObservationBindingDigestV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 28 | `internal/defense/authority_binding_v01.go: unreachable func: EvaluateDefenseAuthorityBindingV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 29 | `internal/defense/authority_binding_v01.go: unreachable func: NewAuthorityIntegrityControlV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 30 | `internal/defense/authority_binding_v01.go: unreachable func: VerifyDefenseAuthorityBindingV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 31 | `internal/defense/authority_binding_v01.go: unreachable func: bindDefenseAuthorityControlV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 32 | `internal/defense/authority_binding_v01.go: unreachable func: bindDefenseAuthorityReceiptV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 33 | `internal/defense/authority_binding_v01.go: unreachable func: bindDefenseAuthorityScenarioV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 34 | `internal/defense/authority_binding_v01.go: unreachable func: containsDefenseAuthorityContainmentReasonV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 35 | `internal/defense/authority_binding_v01.go: unreachable func: decodeDefenseAuthorityBase64V01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 36 | `internal/defense/authority_binding_v01.go: unreachable func: defenseAuthorityArtifactMatchesEvidenceV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 37 | `internal/defense/authority_binding_v01.go: unreachable func: defenseAuthorityCanonicalSHA256V01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 38 | `internal/defense/authority_binding_v01.go: unreachable func: defenseAuthorityEvidenceArtifactSigningBytesV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 39 | `internal/defense/authority_binding_v01.go: unreachable func: equalDefenseAuthorityStringsV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 40 | `internal/defense/authority_binding_v01.go: unreachable func: normalizeDefenseAuthorityBindingEvidenceV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 41 | `internal/defense/authority_binding_v01.go: unreachable func: normalizeDefenseAuthorityEvidenceArtifactV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 42 | `internal/defense/authority_binding_v01.go: unreachable func: normalizeDefenseAuthorityObservationBindingV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 43 | `internal/defense/authority_binding_v01.go: unreachable func: requireDefenseAuthorityEvidenceTrustV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 44 | `internal/defense/authority_binding_v01.go: unreachable func: validDefenseAuthoritySHA256V01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 45 | `internal/defense/authority_binding_v01.go: unreachable func: validateDefenseAuthorityBindingEvidenceV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 46 | `internal/defense/authority_binding_v01.go: unreachable func: validateDefenseAuthorityCaseSemanticsV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 47 | `internal/defense/authority_binding_v01.go: unreachable func: validateDefenseAuthorityEvidenceArtifactShapeV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 48 | `internal/defense/authority_binding_v01.go: unreachable func: validateDefenseAuthorityScenarioCaseBindingV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 49 | `internal/defense/authority_binding_v01.go: unreachable func: verifyDefenseAuthorityEvidenceArtifactV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 50 | `internal/defense/authority_native_execution_v01.go: unreachable func: DefenseAuthorityNativeExecutionAttestationV01.SignEd25519` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 51 | `internal/defense/authority_native_execution_v01.go: unreachable func: defenseAuthorityNativeExecutionAttestationSigningBytesV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 52 | `internal/defense/authority_native_execution_v01.go: unreachable func: normalizeDefenseAuthorityNativeExecutionAttestationV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 53 | `internal/defense/authority_native_execution_v01.go: unreachable func: validateDefenseAuthorityNativeExecutionAttestationShapeV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 54 | `internal/defense/authority_native_execution_v01.go: unreachable func: verifyDefenseAuthorityNativeExecutionAttestationV01` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 55 | `internal/defense/control_contracts.go: unreachable func: BuildDefensiveReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 56 | `internal/defense/control_contracts.go: unreachable func: DeriveEffectAgreement` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 57 | `internal/defense/control_contracts.go: unreachable func: EvaluateDefensivePolicy` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 58 | `internal/defense/control_contracts.go: unreachable func: ValidateFinding` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 59 | `internal/defense/control_contracts.go: unreachable func: contains` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 60 | `internal/defense/control_contracts.go: unreachable func: stringSet` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 61 | `internal/defense/program_authority.go: unreachable func: InspectProgramAuthority` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 62 | `internal/defense/program_authority.go: unreachable func: getProgramAuthorityAccount` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 63 | `internal/defense/program_authority.go: unreachable func: littleEndianUint32` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 64 | `internal/defense/program_authority.go: unreachable func: littleEndianUint64` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 65 | `internal/defense/program_authority.go: unreachable func: parseUpgradeableProgramDataHeader` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 66 | `internal/defense/reproduction.go: unreachable func: ValidateReproductionMarkerOutput` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 67 | `internal/defense/safe_execution.go: unreachable func: EvaluateToolchainPolicy` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 68 | `internal/defense/safe_execution.go: unreachable func: latestToolchainAttestations` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 69 | `internal/defense/safe_execution.go: unreachable func: normalizeToolchainPolicyTools` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 70 | `internal/defense/sentinel_authority_chain_v01.go: unreachable func: VerifySentinelAuthorityChain` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 71 | `internal/defense/sentinel_authority_chain_v01.go: unreachable func: stringSubset` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 72 | `internal/defense/sentinel_authority_chain_v01.go: unreachable func: validateSentinelGrant` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 73 | `internal/defense/sentinel_effect_evidence_v01.go: unreachable func: ObservedDefensiveEffect.Agreement` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 74 | `internal/defense/sentinel_effect_evidence_v01.go: unreachable func: ObservedDefensiveEffect.Digest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 75 | `internal/defense/sentinel_effect_evidence_v01.go: unreachable func: RequireVerifiedDefensiveEffect` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 76 | `internal/executioncontainment/containment.go: unreachable func: CanonicalBytes` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 77 | `internal/executioncontainment/runner.go: unreachable func: ActionArtifact.validFor` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 78 | `internal/executioncontainment/runner.go: unreachable func: EvaluateWithRunner` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 79 | `internal/executionproof/anvil_backend_v03.go: unreachable func: durationOr` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 80 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.blockHash` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 81 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.call` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 82 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.chainID` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 83 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.requireSuccessfulReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 84 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.transactionReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 85 | `internal/executionproof/anvil_backend_v03.go: unreachable func: evmRPCClient.transactionReceiptDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 86 | `internal/executionproof/anvil_backend_v03.go: unreachable func: fileSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 87 | `internal/executionproof/anvil_backend_v03.go: unreachable func: minimalRunnerEnv` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 88 | `internal/executionproof/anvil_backend_v03.go: unreachable func: reserveLocalPort` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 89 | `internal/executionproof/anvil_backend_v03.go: unreachable func: waitForAnvil` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 90 | `internal/executionproof/containment_forward.go: unreachable func: VerifyContainmentAndForwardSafeTransaction` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 91 | `internal/executionproof/containment_forward.go: unreachable func: containmentBindsExactSafeRequest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 92 | `internal/executionproof/evm_payload_v03.go: unreachable func: canonicalEVMAddress` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 93 | `internal/executionproof/evm_payload_v03.go: unreachable func: canonicalEVMPayload` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 94 | `internal/executionproof/evm_payload_v03.go: unreachable func: canonicalHexBytes` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 95 | `internal/executionproof/evm_payload_v03.go: unreachable func: canonicalHexQuantity` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 96 | `internal/executionproof/evm_payload_v03.go: unreachable func: evmPayloadDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 97 | `internal/executionproof/evm_receipt_json_v03.go: unreachable func: canonicalEVMReceipt.UnmarshalJSON` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 98 | `internal/executionproof/fork_binding_v03.go: unreachable func: RunVerifiedForkExecution` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 99 | `internal/executionproof/fork_binding_v03.go: unreachable func: RunVerifiedForkInvariants` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 100 | `internal/executionproof/fork_binding_v03.go: unreachable func: approvedInvariantSetDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 101 | `internal/executionproof/fork_binding_v03.go: unreachable func: blockedVerifiedForkReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 102 | `internal/executionproof/fork_binding_v03.go: unreachable func: canonicalizeApprovedInvariantDefinitions` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 103 | `internal/executionproof/fork_binding_v03.go: unreachable func: prepareVerifiedForkRequest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 104 | `internal/executionproof/fork_binding_v03.go: unreachable func: runVerifiedForkCore` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 105 | `internal/executionproof/fork_binding_v03.go: unreachable func: verifiedForkAdapter.ExecuteForkSimulation` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 106 | `internal/executionproof/fork_canonicality_v03.go: unreachable func: RPCForkCanonicalityVerifier.VerifyCanonical` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 107 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: AssetConservationPolicyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 108 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: BridgeReservePolicyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 109 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.EvaluatePostState` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 110 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.evaluateAssetConservation` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 111 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.evaluateBridgeReserve` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 112 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.evaluatePrivilegedRole` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 113 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.evaluateProxyCodehash` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 114 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PolicyBoundInvariantEvaluator.evaluateTreasuryBound` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 115 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: PrivilegedRolePolicyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 116 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: ProxyCodehashPolicyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 117 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: StaticInvariantPolicyRegistry.ResolveAssetConservation` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 118 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: StaticInvariantPolicyRegistry.ResolveBridgeReserve` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 119 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: StaticInvariantPolicyRegistry.ResolvePrivilegedRole` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 120 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: StaticInvariantPolicyRegistry.ResolveProxyCodehash` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 121 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: StaticInvariantPolicyRegistry.ResolveTreasuryBound` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 122 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: TreasuryBoundPolicyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 123 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: callUint256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 124 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: canonicalBridgeReadProbe` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 125 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: policyDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 126 | `internal/executionproof/invariant_evaluator_v03.go: unreachable func: proxyCodehashEvidenceDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 127 | `internal/executionproof/invariant_runner.go: unreachable func: RunForkInvariants` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 128 | `internal/executionproof/invariant_runner.go: unreachable func: appendUniqueSimulationReason` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 129 | `internal/executionproof/invariant_runner.go: unreachable func: blockedSimulationReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 130 | `internal/executionproof/invariant_runner.go: unreachable func: canonicalizeInvariantChecks` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 131 | `internal/executionproof/invariant_runner.go: unreachable func: canonicalizeRequiredCheckIDs` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 132 | `internal/executionproof/invariant_runner.go: unreachable func: exactRequiredCheckSet` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 133 | `internal/executionproof/invariant_runner.go: unreachable func: normalizeHex32` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 134 | `internal/executionproof/invariant_runner.go: unreachable func: simulationReceiptDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 135 | `internal/executionproof/invariant_runner.go: unreachable func: validForkSimulationRequest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 136 | `internal/executionproof/invariant_runner.go: unreachable func: validInvariantClass` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 137 | `internal/executionproof/proof.go: unreachable func: CanonicalBytes` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 138 | `internal/executionproof/safe_accessor_semantics.go: unreachable func: SafeAccessorSemanticsVerifier.Verify` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 139 | `internal/executionproof/safe_accessor_semantics.go: unreachable func: safeAccessorSimulateCalldataSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 140 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: AnvilSafeSimulationEngine.ExecuteExactSafe` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 141 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: AnvilSafeSimulationEngine.PinnedBlock` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 142 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: AnvilSafeSimulationEngine.RunnerSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 143 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: AnvilSafeSimulationEngine.SnapshotSafe` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 144 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: AnvilSafeSimulationEngine.upstreamClientV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 145 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: abiSelectorHexV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 146 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: abiWordUint64V04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 147 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: addressFromWordBytesV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 148 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: addressFromWordV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 149 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: appendSafeTraceFramesV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 150 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeABIAddressArrayBytesV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 151 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeABIAddressArrayV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 152 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeABIUint64V04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 153 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeABIUintV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 154 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeHexBytesV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 155 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: decodeModulesPageV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 156 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: encodeGetModulesPaginatedV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 157 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: encodeSafeAccessorSimulateV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 158 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: encodeSimulateAndRevertV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 159 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.balanceAtV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 160 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.codeAtV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 161 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.ethCallAtV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 162 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.safeModulesAtV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 163 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.sendNativeMaterializationV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 164 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.storageAtV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 165 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: evmRPCClient.traceSafeSimulationV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 166 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: findSafeAccessorFrameV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 167 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: hexQuantityToDecimalV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 168 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: safeMaterializedEffectDigestV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 169 | `internal/executionproof/safe_anvil_engine_v04.go: unreachable func: snapshotSafeStateV04` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 170 | `internal/executionproof/safe_effect_budget.go: unreachable func: SafeOutflowBudgetVerifier.Verify` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 171 | `internal/executionproof/safe_forward.go: unreachable func: VerifyAndForwardSafeTransaction` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 172 | `internal/executionproof/safe_forward_v03.go: unreachable func: VerifyForkAndForwardSafeTransaction` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 173 | `internal/executionproof/safe_forward_v03.go: unreachable func: blockedForkSigning` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 174 | `internal/executionproof/safe_forward_v03.go: unreachable func: decodeCanonicalHexBytes` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 175 | `internal/executionproof/safe_forward_v03.go: unreachable func: equalAddress` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 176 | `internal/executionproof/safe_forward_v03.go: unreachable func: equalBytes` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 177 | `internal/executionproof/safe_forward_v03.go: unreachable func: forkReceiptMatchesProof` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 178 | `internal/executionproof/safe_forward_v03.go: unreachable func: parseHexUint256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 179 | `internal/executionproof/safe_forward_v03.go: unreachable func: safeTransactionMatchesFork` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 180 | `internal/executionproof/safe_forward_v03.go: unreachable func: sha256Hex` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 181 | `internal/executionproof/safe_isolated_runner.go: unreachable func: SafeIsolatedRunner.Observe` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 182 | `internal/executionproof/safe_isolated_runner.go: unreachable func: equalAddressSets` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 183 | `internal/executionproof/safe_isolated_runner.go: unreachable func: equalSafeAuthority` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 184 | `internal/executionproof/safe_isolated_runner.go: unreachable func: normalizeHexOrAddress` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 185 | `internal/executionproof/safe_isolated_runner.go: unreachable func: normalizeOptionalAddress` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 186 | `internal/executionproof/safe_isolated_runner.go: unreachable func: safeContainmentPolicySHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 187 | `internal/executionproof/safe_isolated_runner.go: unreachable func: validAssetMovement` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 188 | `internal/executionproof/safe_isolated_runner.go: unreachable func: validAuthoritySnapshot` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 189 | `internal/executionproof/safe_isolated_runner.go: unreachable func: validOutflowBound` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 190 | `internal/executionproof/safe_isolated_runner.go: unreachable func: validSHA256Text` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 191 | `internal/executionproof/safe_isolated_runner.go: unreachable func: validateSafeExecutionEvidence` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 192 | `internal/executionproof/safe_pinned_backend.go: unreachable func: PinnedSafeBackend.ExecuteSafe` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 193 | `internal/executionproof/safe_trace_verifier.go: unreachable func: SafeTraceVerifier.Verify` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 194 | `internal/executionproof/safe_trace_verifier.go: unreachable func: safeTraceDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 195 | `internal/executionproof/verified_receipt_v03.go: unreachable func: ValidVerifiedForkReceipt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 196 | `internal/executionproof/verified_receipt_v03.go: unreachable func: canonicalExecutionModel` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 197 | `internal/executionproof/verified_receipt_v03.go: unreachable func: canonicalInvariantEvidenceDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 198 | `internal/executionproof/verified_receipt_v03.go: unreachable func: validExecutionModel` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 199 | `internal/executionproof/verified_receipt_v03.go: unreachable func: validForkExecutionEvidence` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 200 | `internal/executionproof/verified_receipt_v03.go: unreachable func: verifiedForkReceiptDigest` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 201 | `internal/fabric/case_projection_v1.go: unreachable func: BuildWeb3ProjectionV1` | **P0/P1 TEST→PROD REVIEW** |
| 202 | `internal/fabric/case_projection_v1.go: unreachable func: validSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 203 | `internal/handlers/actor_defense_investigation.go: unreachable func: actorDefenseLiquidityRemoval` | **P0/P1 TEST→PROD REVIEW** |
| 204 | `internal/handlers/actor_security_detail.go: unreachable func: Handler.actorSecurityIntelligenceForDetail` | **P0/P1 TEST→PROD REVIEW** |
| 205 | `internal/handlers/arvis_network_probe_intelligence.go: unreachable func: appendArvisProbeEvidenceIfMissing` | **P0/P1 TEST→PROD REVIEW** |
| 206 | `internal/handlers/arvis_network_probe_intelligence.go: unreachable func: attachArvisNetworkProbeIntelligence` | **P0/P1 TEST→PROD REVIEW** |
| 207 | `internal/handlers/autopublish_policy.go: unreachable func: autopublishEnvInt` | **P0/P1 TEST→PROD REVIEW** |
| 208 | `internal/handlers/autopublish_policy.go: unreachable func: autopublishThresholds.policyVersion` | **P0/P1 TEST→PROD REVIEW** |
| 209 | `internal/handlers/autopublish_policy.go: unreachable func: countAutopublishSignalRows` | **P0/P1 TEST→PROD REVIEW** |
| 210 | `internal/handlers/autopublish_policy.go: unreachable func: evaluateAutopublish` | **P0/P1 TEST→PROD REVIEW** |
| 211 | `internal/handlers/autopublish_policy.go: unreachable func: sortedAutopublishReasons` | **P0/P1 TEST→PROD REVIEW** |
| 212 | `internal/handlers/autopublish_worker.go: unreachable func: AutopublishWorkerEnabled` | **P0/P1 TEST→PROD REVIEW** |
| 213 | `internal/handlers/dossier_signal_registry.go: unreachable func: signalSource.signalSourceKey` | **P0/P1 TEST→PROD REVIEW** |
| 214 | `internal/handlers/health.go: unreachable func: resetArvisHealthCache` | **P0/P1 TEST→PROD REVIEW** |
| 215 | `internal/handlers/owner_operations.go: unreachable func: ownerRadarNarrative` | **P0/P1 TEST→PROD REVIEW** |
| 216 | `internal/handlers/transaction_guard_enforcement_permit.go: unreachable func: buildTransactionGuardEnforcementState` | **P0/P1 TEST→PROD REVIEW** |
| 217 | `internal/handlers/transaction_guard_enforcement_permit.go: unreachable func: signTransactionGuardEnforcementPermitWithWitness` | **P0/P1 TEST→PROD REVIEW** |
| 218 | `internal/handlers/transaction_guard_v2.go: unreachable func: guardHTTPStatus` | **P0/P1 TEST→PROD REVIEW** |
| 219 | `internal/handlers/watchlist_monitor.go: unreachable func: watchlistMonitorBatchSize` | **P0/P1 TEST→PROD REVIEW** |
| 220 | `internal/handlers/watchlist_monitor.go: unreachable func: watchlistMonitorEnabled` | **P0/P1 TEST→PROD REVIEW** |
| 221 | `internal/handlers/watchlist_monitor.go: unreachable func: watchlistMonitorInterval` | **P0/P1 TEST→PROD REVIEW** |
| 222 | `internal/http/network_evm_probe_routes.go: unreachable func: networkTargetProbeWithClient` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 223 | `internal/http/network_evm_probe_routes.go: unreachable func: networkTargetProbeWithDependencies` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 224 | `internal/http/server_helpers.go: unreachable func: apiReadiness` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 225 | `internal/networktarget/evm_rollup_finality.go: unreachable func: ProbeEVMRollupFinality` | **P0/P1 TEST→PROD REVIEW** |
| 226 | `internal/networktarget/evm_rollup_finality.go: unreachable func: isSupportedRollupFinalityNetwork` | **P0/P1 TEST→PROD REVIEW** |
| 227 | `internal/networktarget/evm_rollup_finality.go: unreachable func: probeEVMRollupBlockTag` | **P0/P1 TEST→PROD REVIEW** |
| 228 | `internal/radarevent/probe_adapters.go: unreachable func: BuildBitcoinAddressProbeEvent` | **P0/P1 TEST→PROD REVIEW** |
| 229 | `internal/radarevent/probe_adapters.go: unreachable func: BuildEVMAddressProbeEvent` | **P0/P1 TEST→PROD REVIEW** |
| 230 | `internal/router/anthropic_owner.go: unreachable func: OwnerChat` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 231 | `internal/router/anthropic_owner.go: unreachable func: compactAnthropicError` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 232 | `internal/runtimecfg/control_plane_health.go: unreachable func: RecoveredControlPlaneEnvNames` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 233 | `internal/services/actor_defense_correlator.go: unreachable func: actorDefenseCorrelationInterval` | **P0/P1 TEST→PROD REVIEW** |
| 234 | `internal/services/agent_delegation_evidence_chain.go: unreachable func: AdaptSignedAgentDelegationHopEvidenceV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 235 | `internal/services/agent_delegation_evidence_chain.go: unreachable func: BindSignedAgentDelegationEvidenceChainV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 236 | `internal/services/agent_delegation_evidence_chain.go: unreachable func: BuildSignedAgentDelegationEvidenceChainV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 237 | `internal/services/agent_delegation_evidence_chain.go: unreachable func: buildAgentDelegationEvidenceChainV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 238 | `internal/services/agent_delegation_evidence_chain.go: unreachable func: normalizeOptionalAgentSHA256` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 239 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: AdaptSignedAgentIdentityDelegationEvidenceV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 240 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: BindSignedAgentIdentityDelegationEvidenceV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 241 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: agentBindingFinding` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 242 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: agentEventHasSourceDigests` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 243 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: agentEvidenceObservedAt` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 244 | `internal/services/agent_identity_delegation_evidence.go: unreachable func: bindAgentIdentityDelegationProjectionV1` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 245 | `internal/services/arvis_arms.go: unreachable func: buildFinalArm` | **P0/P1 TEST→PROD REVIEW** |
| 246 | `internal/services/arvis_arms.go: unreachable func: finalVerdictFromArm` | **P0/P1 TEST→PROD REVIEW** |
| 247 | `internal/services/arvis_claim_arms.go: unreachable func: buildVerifiedFinalArm` | **P0/P1 TEST→PROD REVIEW** |
| 248 | `internal/services/arvis_stream_signature.go: unreachable func: arvisStreamScopedVerdictSignature` | **P0/P1 TEST→PROD REVIEW** |
| 249 | `internal/services/arvis_transaction_arms.go: unreachable func: buildTransactionIntentProgramArm` | **P0/P1 TEST→PROD REVIEW** |
| 250 | `internal/services/arvis_transaction_arms.go: unreachable func: classifyTransactionIntent` | **P0/P1 TEST→PROD REVIEW** |
| 251 | `internal/services/arvis_transaction_arms.go: unreachable func: hasInstructionContaining` | **P0/P1 TEST→PROD REVIEW** |
| 252 | `internal/services/arvis_transaction_arms.go: unreachable func: nonZeroLamportDeltas` | **P0/P1 TEST→PROD REVIEW** |
| 253 | `internal/services/arvis_transaction_arms.go: unreachable func: nonZeroTokenBalanceChanges` | **P0/P1 TEST→PROD REVIEW** |
| 254 | `internal/services/behavioral_signature_engine.go: unreachable func: BuildBehavioralSignatureReport` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 255 | `internal/services/created_mint_rpc_discovery.go: unreachable func: FetchBoundedRPCCreatedMintDiscovery` | **P0/P1 TEST→PROD REVIEW** |
| 256 | `internal/services/created_mint_rpc_discovery.go: unreachable func: selectCreatedMintRPCSignatures` | **P0/P1 TEST→PROD REVIEW** |
| 257 | `internal/services/evm_approval_intelligence.go: unreachable func: BuildEVMApprovalIntelligence` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 258 | `internal/services/evm_approval_intelligence.go: unreachable func: validHexWord` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 259 | `internal/services/evm_approval_intelligence.go: unreachable func: validTransactionHash` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 260 | `internal/services/evm_approval_intelligence.go: unreachable func: validUnsignedDecimal` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 261 | `internal/services/evm_spender_authority.go: unreachable func: BindEVMApprovalSpenderAuthority` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 262 | `internal/services/global_campaign_effect_attestation.go: unreachable func: BuildGlobalCampaignEffectAttestation` | **P0/P1 TEST→PROD REVIEW** |
| 263 | `internal/services/global_campaign_effect_attestation.go: unreachable func: BuildGlobalCampaignEffectAttestationBinding` | **P0/P1 TEST→PROD REVIEW** |
| 264 | `internal/services/global_campaign_effect_attestation.go: unreachable func: GlobalCampaignEffectAttestationBindingDigest` | **P0/P1 TEST→PROD REVIEW** |
| 265 | `internal/services/global_campaign_effect_attestation.go: unreachable func: ValidateGlobalCampaignEffectAttestation` | **P0/P1 TEST→PROD REVIEW** |
| 266 | `internal/services/global_campaign_effect_attestation.go: unreachable func: globalCampaignEffectAttestationEventMatches` | **P0/P1 TEST→PROD REVIEW** |
| 267 | `internal/services/global_campaign_effect_attestation.go: unreachable func: hashGlobalCampaignEffectAttestation` | **P0/P1 TEST→PROD REVIEW** |
| 268 | `internal/services/global_campaign_effect_attestation.go: unreachable func: rawResponseDigest` | **P0/P1 TEST→PROD REVIEW** |
| 269 | `internal/services/global_campaign_effect_proof.go: unreachable func: NewGlobalCampaignResponseEffectProof` | **P0/P1 TEST→PROD REVIEW** |
| 270 | `internal/services/global_campaign_incident_link.go: unreachable func: LinkGlobalCampaignToIncident` | **P0/P1 TEST→PROD REVIEW** |
| 271 | `internal/services/global_campaign_incident_link.go: unreachable func: LoadGlobalCampaignIncidentLinks` | **P0/P1 TEST→PROD REVIEW** |
| 272 | `internal/services/global_campaign_lookup_merge.go: unreachable func: FindGlobalCampaignCandidates` | **P0/P1 TEST→PROD REVIEW** |
| 273 | `internal/services/global_campaign_lookup_merge.go: unreachable func: MaterializeAndPersistGlobalCampaign` | **P0/P1 TEST→PROD REVIEW** |
| 274 | `internal/services/global_campaign_response_authorization.go: unreachable func: NewGlobalCampaignResponseAuthorization` | **P0/P1 TEST→PROD REVIEW** |
| 275 | `internal/services/global_campaign_response_authorization.go: unreachable func: NewGlobalCampaignResponseProposal` | **P0/P1 TEST→PROD REVIEW** |
| 276 | `internal/services/global_campaign_response_dispatch.go: unreachable func: BuildGlobalCampaignResponseDispatchEnvelope` | **P0/P1 TEST→PROD REVIEW** |
| 277 | `internal/services/global_campaign_response_dispatch.go: unreachable func: ValidateGlobalCampaignResponseDispatchEnvelope` | **P0/P1 TEST→PROD REVIEW** |
| 278 | `internal/services/global_campaign_response_dispatch.go: unreachable func: ValidateGlobalCampaignResponseDispatchEnvelopeShape` | **P0/P1 TEST→PROD REVIEW** |
| 279 | `internal/services/global_campaign_response_dispatch.go: unreachable func: globalCampaignResponseDispatchReplayKey` | **P0/P1 TEST→PROD REVIEW** |
| 280 | `internal/services/global_campaign_response_dispatch.go: unreachable func: hashGlobalCampaignResponseDispatchEnvelope` | **P0/P1 TEST→PROD REVIEW** |
| 281 | `internal/services/global_campaign_response_dispatch.go: unreachable func: normalizeGlobalCampaignResponseDispatchRequest` | **P0/P1 TEST→PROD REVIEW** |
| 282 | `internal/services/global_campaign_response_production_readiness.go: unreachable func: ValidateGlobalCampaignResponseProductionReadiness` | **P0/P1 TEST→PROD REVIEW** |
| 283 | `internal/services/global_campaign_response_production_readiness.go: unreachable func: equalGlobalCampaignResponseReadinessStrings` | **P0/P1 TEST→PROD REVIEW** |
| 284 | `internal/services/global_campaign_state.go: unreachable func: ApplyGlobalCampaignTransition` | **P0/P1 TEST→PROD REVIEW** |
| 285 | `internal/services/global_campaign_store.go: unreachable func: LoadCurrentGlobalCampaign` | **P0/P1 TEST→PROD REVIEW** |
| 286 | `internal/services/global_campaign_store.go: unreachable func: PersistGlobalCampaignRevision` | **P0/P1 TEST→PROD REVIEW** |
| 287 | `internal/services/global_campaign_worker_lease.go: unreachable func: RenewGlobalCampaignWorkerLease` | **P0/P1 TEST→PROD REVIEW** |
| 288 | `internal/services/global_radar_arvis_verdict_reference.go: unreachable func: ProjectARVISSignedVerdictToGlobalRadar` | **P0/P1 TEST→PROD REVIEW** |
| 289 | `internal/services/global_radar_arvis_verdict_reference.go: unreachable func: ProjectVerifiedARVISSignedVerdictToGlobalRadar` | **P0/P1 TEST→PROD REVIEW** |
| 290 | `internal/services/global_radar_liquidity_event.go: unreachable func: BuildSolanaLiquidityRadarEvent` | **P0/P1 TEST→PROD REVIEW** |
| 291 | `internal/services/global_radar_production_readiness.go: unreachable func: BuildGlobalRadarProductionReadiness` | **P0/P1 TEST→PROD REVIEW** |
| 292 | `internal/services/global_radar_production_readiness.go: unreachable func: GlobalRadarProductionReadinessPolicyV1` | **P0/P1 TEST→PROD REVIEW** |
| 293 | `internal/services/global_radar_production_readiness.go: unreachable func: ValidateGlobalRadarProductionReadiness` | **P0/P1 TEST→PROD REVIEW** |
| 294 | `internal/services/global_radar_production_readiness.go: unreachable func: hashGlobalRadarProductionReadiness` | **P0/P1 TEST→PROD REVIEW** |
| 295 | `internal/services/global_radar_production_readiness.go: unreachable func: safeCostPerMillionEvents` | **P0/P1 TEST→PROD REVIEW** |
| 296 | `internal/services/global_radar_production_readiness.go: unreachable func: validateGlobalRadarProductionReadinessPolicy` | **P0/P1 TEST→PROD REVIEW** |
| 297 | `internal/services/global_radar_trusted_verdict_registry.go: unreachable func: GlobalRadarTrustedVerdictRegistry.Resolve` | **P0/P1 TEST→PROD REVIEW** |
| 298 | `internal/services/global_radar_trusted_verdict_registry.go: unreachable func: NewGlobalRadarTrustedVerdictRegistry` | **P0/P1 TEST→PROD REVIEW** |
| 299 | `internal/services/global_radar_trusted_verdict_registry.go: unreachable func: VerifyUnifiedRadarVerdictWithTrustedRegistry` | **P0/P1 TEST→PROD REVIEW** |
| 300 | `internal/services/global_radar_trusted_verdict_registry.go: unreachable func: decodeCanonicalGlobalRadarEd25519` | **P0/P1 TEST→PROD REVIEW** |
| 301 | `internal/services/helius_created_mint_discovery.go: unreachable func: FetchHeliusCreatedMintDiscovery` | **P0/P1 TEST→PROD REVIEW** |
| 302 | `internal/services/holder_wallet_labels.go: unreachable func: ResolveWalletLabel` | **P0/P1 TEST→PROD REVIEW** |
| 303 | `internal/services/holder_wallet_labels.go: unreachable func: resetHeliusWalletIdentityStateForTest` | **P0/P1 TEST→PROD REVIEW** |
| 304 | `internal/services/holder_wallet_labels.go: unreachable func: setHeliusIdentityHTTPClientForTest` | **P0/P1 TEST→PROD REVIEW** |
| 305 | `internal/services/pump_high_volume_radar.go: unreachable func: pumpHighVolumeAttemptCooldown` | **P0/P1 TEST→PROD REVIEW** |
| 306 | `internal/services/pump_high_volume_radar.go: unreachable func: pumpHighVolumeMaxReportsPerCycle` | **P0/P1 TEST→PROD REVIEW** |
| 307 | `internal/services/pump_trade_ledger.go: unreachable func: PumpTradeLedgerWriter.EnqueuePumpPortal` | **P0/P1 TEST→PROD REVIEW** |
| 308 | `internal/services/pump_trade_ledger.go: unreachable func: tokenTradeEventFromPumpPortal` | **P0/P1 TEST→PROD REVIEW** |
| 309 | `internal/services/pumpportal_client.go: unreachable func: NewPumpPortalClient` | **P0/P1 TEST→PROD REVIEW** |
| 310 | `internal/services/pumpportal_client.go: unreachable func: PumpPortalClient.rememberTradeMint` | **P0/P1 TEST→PROD REVIEW** |
| 311 | `internal/services/pumpportal_client.go: unreachable func: firstPumpPortalFloat` | **P0/P1 TEST→PROD REVIEW** |
| 312 | `internal/services/pumpportal_client.go: unreachable func: firstPumpPortalInt64` | **P0/P1 TEST→PROD REVIEW** |
| 313 | `internal/services/pumpportal_client.go: unreachable func: firstPumpPortalString` | **P0/P1 TEST→PROD REVIEW** |
| 314 | `internal/services/pumpportal_client.go: unreachable func: firstPumpPortalTime` | **P0/P1 TEST→PROD REVIEW** |
| 315 | `internal/services/pumpportal_client.go: unreachable func: normalizePumpPortalTradeSide` | **P0/P1 TEST→PROD REVIEW** |
| 316 | `internal/services/pumpportal_client.go: unreachable func: parsePumpPortalEvent` | **P0/P1 TEST→PROD REVIEW** |
| 317 | `internal/services/pumpportal_client.go: unreachable func: pumpPortalTradeSubscriptionLimit` | **P0/P1 TEST→PROD REVIEW** |
| 318 | `internal/services/pumpportal_client.go: unreachable func: pumpPortalUnixTime` | **P0/P1 TEST→PROD REVIEW** |
| 319 | `internal/services/pumpportal_client.go: unreachable func: sanitizePumpPortalRaw` | **P0/P1 TEST→PROD REVIEW** |
| 320 | `internal/services/pumpportal_durable_inbox.go: unreachable func: pumpPortalInboxEventKey` | **P0/P1 TEST→PROD REVIEW** |
| 321 | `internal/services/pumpportal_observable_client.go: unreachable func: classifyPumpPortalProviderNotice` | **P0/P1 TEST→PROD REVIEW** |
| 322 | `internal/services/pumpportal_radar_adapter.go: unreachable func: pumpPortalEventType` | **P0/P1 TEST→PROD REVIEW** |
| 323 | `internal/services/pumpportal_radar_adapter.go: unreachable func: resolvePumpPortalMint` | **P0/P1 TEST→PROD REVIEW** |
| 324 | `internal/services/security_incident_corpus.go: unreachable func: securityIncidentCorpusIncidentKey` | **P0/P1 TEST→PROD REVIEW** |
| 325 | `internal/services/security_incident_corpus.go: unreachable func: securityIncidentCorpusRecordFromCandidate` | **P0/P1 TEST→PROD REVIEW** |
| 326 | `internal/services/security_incident_corpus.go: unreachable func: securityIncidentCorpusRecordHash` | **P0/P1 TEST→PROD REVIEW** |
| 327 | `internal/services/security_incident_corpus.go: unreachable func: securityIncidentMaterialRiskLevel` | **P0/P1 TEST→PROD REVIEW** |
| 328 | `internal/services/security_radar_evidence_gate.go: unreachable func: EvidenceBackedFinalSecurityRadarVerdict` | **P0/P1 TEST→PROD REVIEW** |
| 329 | `internal/services/security_radar_retention_worker.go: unreachable func: retentionArchiveQuery` | **P0/P1 TEST→PROD REVIEW** |
| 330 | `internal/services/security_radar_retention_worker.go: unreachable func: retentionDeadlineError` | **P0/P1 TEST→PROD REVIEW** |
| 331 | `internal/services/security_radar_retention_worker.go: unreachable func: retentionInitialBatchSize` | **P0/P1 TEST→PROD REVIEW** |
| 332 | `internal/services/security_radar_retention_worker.go: unreachable func: retentionNextBatchSize` | **P0/P1 TEST→PROD REVIEW** |
| 333 | `internal/services/security_radars.go: unreachable func: FinalSecurityRadarVerdict` | **P0/P1 TEST→PROD REVIEW** |
| 334 | `internal/services/solana_rpc_batch.go: unreachable func: resetSolanaRPCBatchCircuitForTest` | **P0/P1 TEST→PROD REVIEW** |
| 335 | `internal/services/solana_rpc_budget.go: unreachable func: resetSolanaRPCBudgetForTest` | **P0/P1 TEST→PROD REVIEW** |
| 336 | `internal/services/solana_rpc_client.go: unreachable func: resetSolanaRPCCachesForTest` | **P0/P1 TEST→PROD REVIEW** |
| 337 | `internal/services/solana_rpc_signature_guard.go: unreachable func: resetSolanaRPCSignaturePressureForTest` | **P0/P1 TEST→PROD REVIEW** |
| 338 | `internal/services/solana_transaction_guard_accounts.go: unreachable func: SolanaTokenAccountRawAmount` | **P0/P1 TEST→PROD REVIEW** |
| 339 | `internal/services/solana_validator_telemetry.go: unreachable func: ProbeSolanaValidatorTelemetry` | **P0/P1 TEST→PROD REVIEW** |
| 340 | `internal/services/solana_validator_telemetry.go: unreachable func: ProjectSolanaValidatorTelemetryToGlobalRadar` | **P0/P1 TEST→PROD REVIEW** |
| 341 | `internal/services/solana_validator_telemetry.go: unreachable func: totalSolanaActivatedStake` | **P0/P1 TEST→PROD REVIEW** |
| 342 | `internal/services/threat_operational_memory.go: unreachable func: AugmentThreatAnticipationWithOperationalMemory` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 343 | `internal/services/threat_operational_memory.go: unreachable func: normalizeThreatOperationalMemoryStatus` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 344 | `internal/services/threat_operational_memory.go: unreachable func: threatOperationalMemorySummary` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 345 | `internal/services/unified_radar_c006.go: unreachable func: ApplyCrossTokenCreatorHolderTransferRuleV120` | **P0/P1 TEST→PROD REVIEW** |
| 346 | `internal/services/unified_radar_c006.go: unreachable func: distinctC006OtherTokens` | **P0/P1 TEST→PROD REVIEW** |
| 347 | `internal/services/unified_security_contract.go: unreachable func: BuildTrustBoundaryTransition` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 348 | `internal/web3/solana_rpc_governor.go: unreachable func: ResetSolanaRPCProviderGovernorForTest` | **P0/P1 TEST→PROD REVIEW** |
| 349 | `internal/web3/solana_rpc_signature_pressure.go: unreachable func: resetSolanaRPCSignaturePressureForTest` | **P0/P1 TEST→PROD REVIEW** |
| 350 | `internal/webhooks/crypto.go: unreachable func: VerifySignature` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 351 | `pkg/utils/crypto.go: unreachable func: ComparePassword` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 352 | `pkg/utils/crypto.go: unreachable func: HashPassword` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |
| 353 | `pkg/utils/crypto.go: unreachable func: IsArgon2Hash` | **P2 INTENTIONAL-TEST-ONLY REVIEW** |

## C — Unreferenced/test-only public assets (32)

| # | Asset | Production disposition |
|---:|---|---|
| 1 | `TESTONLY /agent-api.html` | **P1 TESTONLY SURFACE REVIEW** |
| 2 | `TESTONLY /architecture.html` | **P1 TESTONLY SURFACE REVIEW** |
| 3 | `TESTONLY /chains.html` | **P1 TESTONLY SURFACE REVIEW** |
| 4 | `TESTONLY /css/evm-authority-desk.css` | **P1 TESTONLY SURFACE REVIEW** |
| 5 | `TESTONLY /css/koschei-scan-premium.css` | **P1 TESTONLY SURFACE REVIEW** |
| 6 | `TESTONLY /css/premium-evidence-matrix.css` | **P1 TESTONLY SURFACE REVIEW** |
| 7 | `TESTONLY /full-scan-contract-v1.json` | **KEEP PUBLIC · LIVE SMOKE CONTRACT** |
| 8 | `ORPHAN   /js/__fixtures__/cfpk-live-scan.json` | **P0 BLOCK HTTP · KEEP FILE** |
| 9 | `ORPHAN   /js/__fixtures__/historical-scan.json` | **P0 BLOCK HTTP · KEEP FILE** |
| 10 | `TESTONLY /js/__tests__/arvis-canonical-projection.test.mjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 11 | `ORPHAN   /js/__tests__/crypto-brief.test.cjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 12 | `ORPHAN   /js/__tests__/customer-scan-entry.test.cjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 13 | `ORPHAN   /js/__tests__/evidence-health.test.cjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 14 | `ORPHAN   /js/__tests__/owner-global-campaigns.test.cjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 15 | `ORPHAN   /js/__tests__/verdict-card-analysis-summary.test.mjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 16 | `ORPHAN   /js/__tests__/verdict-card-market-context.test.mjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 17 | `ORPHAN   /js/__tests__/verdict-card.test.mjs` | **P0 BLOCK HTTP · KEEP FILE** |
| 18 | `ORPHAN   /js/customer-investigation-ux-v2.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 19 | `TESTONLY /js/customer-transaction-preflight-v1.js` | **P1 TESTONLY SURFACE REVIEW** |
| 20 | `ORPHAN   /js/evm-authority-desk.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 21 | `ORPHAN   /js/lp-control-evidence-card.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 22 | `ORPHAN   /js/premium-evidence-matrix.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 23 | `ORPHAN   /js/public-scan-request-guard.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 24 | `ORPHAN   /js/unified-live-evidence-card.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 25 | `ORPHAN   /js/verdict-card-evidence-refs.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 26 | `ORPHAN   /js/verdict-card-market-context.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 27 | `TESTONLY /pay-per-tool.html` | **P1 TESTONLY SURFACE REVIEW** |
| 28 | `ORPHAN   /sdk/koschei-shield.js` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 29 | `ORPHAN   /solana-security-tools.html` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 30 | `ORPHAN   /token-2022-scanner.html` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 31 | `ORPHAN   /token-vesting.html` | **P1 VERIFY/REGISTER OR BLOCK HTTP** |
| 32 | `TESTONLY /validation-key.txt` | **P0 SECURITY REVIEW · KEEP FILE** |

## D — Frontend/API route mismatches (6)

| # | Route | Current-main result |
|---:|---|---|
| 1 | `/api/dao/proposal-risk` | **DORMANT koschei-modern.js reference · canonical route decision required** |
| 2 | `/api/liquidity/analyze` | **DORMANT koschei-modern.js reference · canonical route decision required** |
| 3 | `/api/mev/analyze` | **DORMANT koschei-modern.js reference · canonical route decision required** |
| 4 | `/api/owner/kosch-access` | **RETIRED · current contract forbids legacy route** |
| 5 | `/api/public/tool-prices` | **NO CURRENT MAIN REFERENCE FOUND** |
| 6 | `/api/wallet/score` | **DORMANT koschei-modern.js reference · canonical route decision required** |

## Immediate production order

1. Close live scan/RPC acceptance and enable bounded background Radar.
2. Close owner + customer panel acceptance and Professional entitlement lifecycle.
3. Block test fixtures/tests/internal-only public assets without deleting repository files.
4. Resolve dormant frontend/API mismatches by canonical routing or explicit preserved-retired status.
5. Wire product-critical A/B families: transaction arms, PumpPortal ingestion/ledger/radar, actor correlation, watchlist monitor, retention, LP/MEV and creator/funding intelligence.
6. Re-run deadcode against production entrypoints and update every ledger row; no row is removed from this ledger until its disposition is proven.