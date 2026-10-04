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

## Validation status

- `gofmt`: clean for modified Go files.
- static top-level `Start*` call scan: no unowned production start function remains except the intentionally superseded legacy retention worker.
- full local Go test: blocked in this execution environment because the repository requires Go `1.26.7`, while the local toolchain is `1.23.2`, and external toolchain/module downloads are unavailable. CI must provide the final compiler/race-detector proof.
