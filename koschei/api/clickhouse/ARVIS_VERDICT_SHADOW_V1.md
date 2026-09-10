# ARVIS Verdict Shadow V1

This layer is additive. It does not replace ARVIS Radar, change its scoring/verdict logic, claim queue ownership, or move live customer reads to ClickHouse.

## Purpose

Copy the committed `security_radar_verdicts` state into ClickHouse as queryable intelligence memory while the existing ARVIS runtime remains authoritative.

The one-shot copier reads PostgreSQL through a repeatable-read, read-only snapshot and writes `koschei_web3.arvis_verdict_snapshots`. It verifies both row-count parity and a bounded content fingerprint before reporting success.

## Source contract

Each snapshot preserves:

- committed verdict UUID and optional source event UUID
- module, target, target type and network
- grade, risk index, risk level, verdict and recommendation
- evidence list plus SHA-256
- canonicalized signals JSON plus SHA-256
- rule version and signed state
- signature, source, event type and provider provenance
- original `created_at` and `updated_at` timestamps

Chain-native target and signature strings remain case-sensitive.

## ClickHouse model

`ReplacingMergeTree(source_version)` is used only for convergence of repeated shadow copies of the same committed verdict row. `source_version` is derived from PostgreSQL `updated_at` at microsecond precision. This is not a uniqueness or transactional-lock claim.

No partition is introduced yet. No skipping index is introduced without measured workload evidence.

## Fail-closed parity

A shadow run fails if:

- the source window exceeds the configured row cap
- required identity/timestamps are missing
- source evidence/signals cannot be normalized
- an insert fails
- ClickHouse distinct-row count differs from the source snapshot
- the bounded content fingerprint differs
- the ClickHouse read exceeds the configured parity cap

## Explicit non-goals

This change does not:

- modify `SecurityRadarStore.InsertVerdict`
- modify `arvis_stream_processing`
- replace PostgreSQL queue/lease/retry semantics
- change ARVIS decisions or customer responses
- create live ClickHouse customer reads
- apply schema to ClickHouse Cloud automatically
- change ClickHouse plan, replicas, RAM/vCPU, TDE or billing configuration
- modify Koschei Sentinel or Koschei Lang

Cutover, if ever authorized, is a separate validated production decision.
