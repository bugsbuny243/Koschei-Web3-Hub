# Radar runtime integration — 2026-10-04

## Goal

Close implementation-without-runtime gaps around Koschei Security Radar, Global Radar, and Global Campaign without weakening the existing evidence and authority boundaries.

## Runtime wiring added

`startBackgroundRuntime` now owns the lifecycle of these previously defined but unstarted top-level workers:

- customer webhook delivery (`webhooks.StartDeliveryWorker`);
- Actor Defense correlation (`services.StartActorDefenseCorrelator`);
- PumpPortal Radar adapter (`services.StartPumpPortalRadarIfEnabled`);
- Global Campaign Radar runtime (`services.StartGlobalCampaignRadarRuntime`);
- watchlist monitor (`handlers.StartWatchlistMonitor`);
- dossier autopublish worker (`handlers.StartAutopublishWorker`).

Every worker keeps its existing feature/config gate. Wiring a worker into the lifecycle does not force-enable it.

## Security Radar -> Global Radar -> Global Campaign

The Global Campaign Radar runtime consumes only signed, evidence-backed `final_verdict_engine` rows from `security_radar_verdicts`.

For each eligible verdict it:

1. resolves the target through the canonical intelligence subject classifier;
2. creates a canonical `GlobalRadarObservation` with `evidence_only_no_verdict_created` decision state;
3. builds and re-validates a canonical `GlobalRadarSnapshot`;
4. validates the snapshot through `BuildGlobalCampaignRadarProjection`;
5. materializes/persists correlation-only Global Campaign context using the canonical observation reference;
6. records the handoff in `global_campaign_radar_ingest` so backlog processing is durable and idempotent.

Unsupported/unclassified targets are durably recorded as `skipped`; they are not silently coerced into another chain family.

The runtime can be controlled explicitly with `KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED`; when unset it follows the existing automatic background-scanning gate. `KOSCHEI_GLOBAL_CAMPAIGN_INTERVAL` is bounded to 15 seconds..15 minutes and defaults to one minute. `KOSCHEI_GLOBAL_CAMPAIGN_BATCH_SIZE` is bounded to 1..500 and defaults to 50.

## Owner Radar visibility

Validated current Global Campaign projections are now included in the existing owner Radar overview (`GET /api/owner/arvis`) as `global_campaigns` with `global_campaign_authority=correlation_only`.

The read path calls the same stored-payload validation used by the campaign store. Metadata/payload/hash disagreement fails closed rather than surfacing partially trusted state. No new write route or response/containment command is introduced.

## Solana validator telemetry

The existing Solana validator telemetry probe and `ProjectSolanaValidatorTelemetryToGlobalRadar` projector are now part of the opt-in Global Radar background telemetry path.

When `KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED=1` and `solana-mainnet` is explicitly present in `KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS`, the runtime requires an explicitly configured Solana RPC endpoint and performs:

`getVoteAccounts -> ProbeSolanaValidatorTelemetry -> ProjectSolanaValidatorTelemetryToGlobalRadar -> BuildGlobalRadarSnapshot -> configured Global Radar snapshot sink`

The projector remains evidence-only. It does not create a decentralization score, risk grade, verdict, identity claim, or containment authority.

Solana validator telemetry is intentionally not added to the block-head ingest switch because the current head-ingest implementation supports EVM/Bitcoin block lineage contracts, not Solana slot/block semantics.

## Authority boundary preserved

This integration does not:

- create or modify an ARVIS grade or verdict;
- re-sign a verdict;
- infer common control, real-world identity, malicious intent, or wrongdoing;
- grant containment or production response authority;
- treat `real_onchain_evidence` / `real_offchain_evidence` as VERIFIED unless the existing verdict marks evidence verified.

Global Campaign remains correlation context only.

## Worker fencing

The new Global Campaign Radar runtime uses the existing `global_campaign_worker_leases` contract for:

- acquisition;
- renewal;
- per-item lease assertion;
- fencing-token enforcement;
- bounded best-effort release on shutdown.

This turns the previously standalone lease implementation into a real runtime boundary.

## Intentionally not wired twice

`StartSecurityRadarRetentionWorker` remains unstarted because `StartSecurityRadarRetentionWorkerV2` is already owned by `StartSecurityRadarWatcher`. The v2 implementation explicitly keeps the legacy worker only as a rollback path. Running both would duplicate archive/delete work.

## Implemented projectors intentionally left without fabricated inputs

Two repo-level projectors remain deliberately outside the production path because their required authority/evidence prerequisites do not currently exist as a safe runtime source:

- `BuildSolanaLiquidityRadarEvent` requires two successive, available market snapshots for the same mint with increasing observation time. The current repo does not provide a durable previous/current market-snapshot source for that contract. A single snapshot is not converted into a fake delta.
- `ProjectVerifiedARVISSignedVerdictToGlobalRadar` requires the Unified Radar signed-verdict domain plus a trusted Ed25519 public-key registry. Persisted Security Radar verdicts use a different signing payload/domain; their signatures are not reinterpreted as Unified Radar signatures.

These are prerequisite gaps, not forgotten boot calls. They must be wired only after the required canonical source/trust material exists.

The production response forwarder also remains fail-closed. Radar correlation does not silently enable containment execution.

## Validation status

- `gofmt`: clean for modified Go files.
- static top-level `Start*` call scan: no unowned production start function remains except the intentionally superseded legacy retention worker.
- dedicated GitHub Actions acceptance uses the repository Go toolchain and the race detector for Campaign runtime, Solana validator telemetry, runtime config, and owner Radar compilation.
- earlier dedicated acceptance runs for the Campaign runtime and Solana extension completed successfully under the repository toolchain; the PR head remains subject to the same CI gate after every follow-up change.
- full local Go test remains unavailable in this execution environment because the repository requires Go `1.26.7`, while the local toolchain is `1.23.2`, and external toolchain/module downloads are unavailable.
