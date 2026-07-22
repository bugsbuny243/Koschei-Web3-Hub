# Koschei Solana Defense Intelligence OS — Canonical Roadmap

This file is the durable phase index for the Defense OS. Conversation history is not the source of truth; merged code, migrations, tests and the phase documents are.

## Constitutional boundary

Across every phase:

- the signed deterministic Koschei investigation verdict remains the only verdict authority;
- Defense OS evidence uses `verdict_authority=false`;
- missing evidence remains pending, unavailable or insufficient;
- AI may explain or propose, but cannot create a finding, raise or lower a grade, approve a patch or claim proof-of-fix;
- no Defense OS path may hold a wallet key, sign a transaction, submit a mainnet transaction or deploy a production patch;
- source, IDL, bytecode, toolchain, execution and reproduction evidence remain distinct and independently attributable.

## Completed phases

### Phase 1 — Immutable Defense Agent Shadow Runtime

Read-only agent and tool envelopes, immutable run records, Program Archaeologist, Static Analyzer and Reproduction Agent roles, and a deterministic program-surface resolver attached in shadow mode.

Status: **complete**.

### Phase 2 — Immutable Artifact Intake + Knowledge Fabric

Immutable source bundles, source manifests, Anchor IDLs, sBPF artifacts, build manifests and knowledge documents, all bound to hashes and provenance.

Status: **complete**.

### Phase 3 — Program Security Lab

Temporal program/instruction/account graph and conservative deterministic Solana/Anchor detector rules. Static observations remain hypotheses until stronger evidence exists.

Status: **complete**.

### Phase 4 — Local Verification + Review-Only Repair

Bounded source sandbox, fixed command allowlist, structured patch proposals and immutable owner approval. Generic build/test success remains partial evidence.

Status: **complete**.

### Phase 5 — Learning Flywheel

Immutable benchmark cases, deterministic evaluation, non-production defensive mutations and reviewed training-example export.

Status: **complete**.

### Phase 6 — Deployed Bytecode Verification

Read-only Program/ProgramData resolution, deployment and upgrade-authority evidence, canonical sBPF hashing and optional build-manifest byte equality checks.

Status: **complete**.

### Phase 7 — Exact-Commit Source Import

Bounded public GitHub import pinned to an exact commit with host, redirect, archive, path, file-count and content safety controls.

Status: **complete**.

### Phase 8 — Separate Defense Worker

Durable PostgreSQL queue, leases, stale recovery, append-only worker events and a separate worker process. The web service never executes imported source.

Status: **complete**.

### Phase 9 — Versioned Paired Reproduction

Owner-approved reproduction invariants, exact finding/source/command binding, distinct unpatched and patched runs, exact marker requirements and immutable proof records.

Status: **complete**.

### Phase 10 — Program Deployment Sentinel

Read-only scheduled deployment snapshots and immutable change events for loader, ProgramData, bytecode, upgrade authority and manifest-match state.

Status: **complete**.

### Phase 11 — Anchor Harness Planner + Toolchain Attestation

Deterministic non-executable harness plans derived from Anchor IDLs, human-confirmation invariant templates and immutable worker toolchain probes.

Every Phase 11 plan remains:

```text
execution_ready=false
manual_guidance_required=true
verdict_authority=false
```

Status: **complete**.

### Phase 12A — Pinned Toolchain + Fail-Closed Execution Profile

Exact executable SHA-256 evidence, immutable worker-image binding, owner-confirmed invariants, fixed LiteSVM/Trident command policies, immutable ready/blocked profiles, stale-image rejection and execution-time binary rehash authorization.

Phase 12A exposes no harness execution action.

Status: **complete**.

## Active phase

### Phase 12B — Deterministic Offline Harness Materialization

Phase 12B consumes one ready LiteSVM profile and its owner-prepared immutable harness bundle. It produces a second normalized immutable artifact without downloading dependencies or executing source.

Required deliverables:

1. root `Cargo.toml` and immutable `Cargo.lock` validation;
2. direct LiteSVM dependency and matching lock-package evidence;
3. rejection of Git dependencies and path dependencies that escape the bundle;
4. deterministic path and text normalization;
5. raw SHA-256 for every normalized file, Cargo manifest and Cargo lock;
6. generated timestamp-free `koschei/materialization.json` bound to profile and source hashes;
7. immutable materialized source-bundle artifact with `artifact_role=materialized_harness`;
8. immutable migration-backed materialization record;
9. explicit `dependency_resolution=false`, `source_executed=false`, `harness_executed=false` and `verdict_authority=false`;
10. no command queue or worker execution route in this subphase.

Status: **in progress**.

## Remaining Phase 12 work

### Phase 12C — First Isolated Deterministic LiteSVM Run

Planned. A separate worker action may consume only a Phase 12A-authorized profile and a Phase 12B materialized artifact. It must revalidate the live worker image and executable hashes immediately before launching the fixed offline command.

The run must persist bounded stdout/stderr, exit state, duration, input hashes, materialization hash, worker identity, tool pins and explicit no-mainnet evidence.

## Planned phases after Phase 12

### Phase 13 — Stateful Adversarial Sequence Engine

Pinned Trident or equivalent stateful fuzzing, bounded sequence grammars, deterministic seeds, corpus retention and reproducible crash/minimal-sequence evidence.

### Phase 14 — Reachability and Asset-Impact Proof Layer

Bind a static finding to a concrete instruction path, controlled account state and measurable unauthorized state or asset delta. Capability alone is not exploitability.

### Phase 15 — Differential Patch Verification

Run exact baseline and patched programs against the same immutable fixtures, seeds and invariants. A patch must remove the target failure without violating accepted invariants.

### Phase 16 — Continuous Program Defense

Sentinel changes may queue bounded re-analysis and approved regression suites. Deployment changes never silently inherit prior proof.

### Phase 17 — Human Review and Release Governance

Review queues, evidence signing, separation of duties, approval thresholds and explicit export of a technical security dossier. Koschei still does not deploy patches.

## Phase transition rule

A phase is complete only when its code, migration, tests, documentation and evidence boundaries are merged and the required CI gates pass. Feature presence alone does not complete a phase.
