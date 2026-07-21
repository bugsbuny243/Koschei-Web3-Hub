# Koschei Solana Defense Intelligence OS — Master Roadmap

This document is the durable source of truth for the Defense OS phase sequence. It reconstructs the implemented GitHub history and makes future phases explicit so the roadmap is not lost inside a chat transcript.

## Constitutional boundary

The roadmap is governed by `ACTOR_INVESTIGATION_ENGINE.md` v1.0, especially:

- §1 question 10: separate verified, observed, inferred and unknown claims;
- §3: preserve evidence levels and graceful degradation;
- §4: serious claims require complete evidence rows;
- §5: only deterministic versioned rules may affect a verdict.

Defense OS output has `verdict_authority=false`. It may provide technical evidence to an investigation, but it cannot create, raise, lower or override an ARVIS or Unified Radar grade. No Defense OS phase may sign or submit a mainnet transaction, obtain a wallet private key, or apply a production patch automatically.

## Status vocabulary

- **COMPLETE** — merged implementation and persistence contract exist.
- **IN PROGRESS** — implementation is actively being built and is not a completed product claim.
- **PLANNED** — architectural intent only; no production capability claim.

## Implemented foundation

### Phase 1 — Immutable Defense Agent Shadow Runtime — COMPLETE

Read-only Program Archaeologist, Static Analyzer and Reproduction Agent roles; hashed tool envelopes; immutable agent/tool records; no execution or verdict authority.

### Phase 2 — Immutable Artifact Intake + Knowledge Fabric — COMPLETE

Hash-addressed source bundles, manifests, Anchor IDLs, sBPF artifacts and knowledge documents with provenance and trust metadata.

### Phase 3 — Program Security Lab — COMPLETE

Temporal program/instruction/account graph and conservative deterministic Solana/Anchor surface detectors. Static findings remain hypotheses until stronger evidence exists.

### Phase 4 — Local Verification + Review-Only Repair — COMPLETE

Bounded command sandbox, schema-bound repair proposals and immutable owner approval. Generic build success is not proof of exploitability or proof-of-fix.

### Phase 5 — Learning Flywheel — COMPLETE

Immutable benchmark cases, deterministic precision/recall evaluation, defensive synthetic mutations and human-reviewed dataset export.

### Phase 6 — Deployed Bytecode Verification — COMPLETE

Upgradeable-loader resolution, ProgramData and upgrade-authority evidence, canonical sBPF hashes and optional build-manifest byte equality checks.

### Phase 7 — Exact-Commit Public Source Import — COMPLETE

Bounded GitHub archive import pinned to an exact commit SHA with strict host, redirect, path, binary and size policies. Imported source is not executed.

### Phase 8 — Isolated Defense Worker — COMPLETE

Separate worker process and image, durable PostgreSQL queue, leases, stale recovery and fixed command allowlisting. The web service never executes source.

### Phase 9 — Versioned Paired Reproduction — COMPLETE

Exact baseline/patched command and marker binding. A proof-of-fix requires both immutable runs; ordinary tests are insufficient.

### Phase 10 — Program Deployment Sentinel — COMPLETE

Read-only monitoring of loader, ProgramData, canonical bytecode, upgrade authority and build-manifest state with immutable change events.

### Phase 11 — Anchor Harness Planner + Toolchain Attestation — COMPLETE

Deterministic non-executable plans from Anchor IDL metadata, human-confirmation invariant templates and bounded worker probes for Rust, Cargo, Solana, Anchor and Trident.

## Execution and fuzzing track

### Phase 12 — Pinned Toolchain and Fail-Closed Execution Gate — IN PROGRESS

Phase 12 is split so planning cannot silently become execution.

#### Phase 12A — Immutable execution profile foundation

- hash the exact executable used for every toolchain attestation;
- bind attestations to an operator-supplied immutable worker image digest;
- require an immutable harness source bundle tied to one Phase 11 plan;
- require explicit owner-confirmed invariant statements;
- create a deterministic engine command policy;
- persist a versioned execution profile as either `ready` or `blocked`;
- reject worker authorization when worker ID, image digest or profile state differs;
- provide no execution route in this subphase.

#### Phase 12B — Deterministic harness materialization

PLANNED. Materialize a reviewable LiteSVM-compatible project from a manually prepared immutable harness bundle. Record generated files and dependency locks as immutable artifacts. Network access remains disabled.

#### Phase 12C — First isolated deterministic run

PLANNED. Add a worker action that can consume only a Phase 12A-authorized profile and a Phase 12B materialized harness. Persist bounded stdout/stderr, command, duration, exit status, input hashes and worker/toolchain pins.

### Phase 13 — LiteSVM Invariant Execution — PLANNED

Execute confirmed no-panic, signer-substitution, read-only-account and allowed-state-transition invariants. A passing run is technical evidence only; it does not establish complete program safety.

### Phase 14 — Stateful Trident Fuzzing — PLANNED

Generate bounded stateful sequences from confirmed grammars, persist seeds and minimized failure traces, and distinguish environment failures from program failures.

### Phase 15 — Differential Upgrade Regression — PLANNED

Run identical invariant corpora against baseline and candidate deployments/source builds. Record state/output differences without inferring intent.

### Phase 16 — Failure Minimization and Reproducible PoC — PLANNED

Minimize a failing instruction sequence while preserving exact artifacts, accounts, arguments, seed and toolchain pins. Any exploitability claim still requires human review and evidence-policy checks.

### Phase 17 — Reachability and Asset-Impact Evidence — PLANNED

Connect reproduced technical behavior to deployed bytecode, reachable instruction surfaces and bounded asset-impact evidence. Capability is not intent and external labels do not prove control.

### Phase 18 — Review-Only Repair Validation — PLANNED

Bind a proposed patch to the reproduced failure, run paired baseline/patched evidence and produce a reviewable proof package. Koschei does not apply the patch to production.

### Phase 19 — Continuous Program Defense — PLANNED

Sentinel-triggered regression scheduling after verified deployment changes, with tenant-scoped alerts and deduplicated immutable evidence.

### Phase 20 — Production Acceptance and Governance — PLANNED

Real-program acceptance corpus, false-positive review, resource ceilings, rollback drills, operator runbooks and explicit enablement gates. No phase becomes customer-visible merely because code exists.

## Current checkpoint

Phases 1–11 are merged. Phase 12A is the active work item. Until 12B and 12C are implemented and validated, Koschei must continue to report:

```text
harness_execution_available=false
mainnet_transaction_sent=false
verdict_authority=false
```
