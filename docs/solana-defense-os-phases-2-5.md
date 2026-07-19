# Koschei Solana Defense Intelligence OS — Phases 2–5

This build extends the Phase 1 shadow runtime without replacing the existing ARVIS, actor investigation, LP, market, threat anticipation or signed deterministic verdict paths.

Applicable product contract: `ACTOR_INVESTIGATION_ENGINE.md` §1 questions 8–10, §3 evidence levels, §4 evidence rows, §5 deterministic verdict and §6 persistent memory. Actor ruleset v1.0; unified Radar ruleset `koschei-unified-radar-rules-v1.0.0`.

## Constitutional boundary

- Program-security findings have `verdict_authority=false`.
- AI may plan or propose a patch but cannot create, raise, lower or override a grade.
- Static observations remain hypotheses until reachability and reproduction are independently established.
- No Defense OS path can sign or send a mainnet transaction.
- Patch proposals are review-only, require an append-only owner approval record, and are never applied to production by the API.
- The sandbox is disabled by default and executes only fixed allowlisted commands in an ephemeral directory.
- Synthetic mutations are marked `synthetic_source_bundle` and `production_eligible=false`.

## Phase 2 — Artifact Intake and Knowledge Fabric

Owner-only endpoints:

- `GET|POST /api/owner/defense/artifacts`
- `GET|POST /api/owner/defense/knowledge`

Accepted immutable artifact types:

- `source_bundle`: JSON object mapping safe relative paths to UTF-8 file contents
- `source_manifest`
- `anchor_idl`
- `sbpf_bytecode`
- `sbpf_manifest`
- `knowledge_document`
- `synthetic_source_bundle`

Each artifact is bound to:

- program ID and network
- SHA-256 content hash
- source URI and commit when supplied
- framework, framework version and runtime version
- trust level and verification status
- immutable artifact reference

Knowledge documents support PostgreSQL full-text retrieval. Together embeddings can be enabled independently; vectors are stored as PostgreSQL arrays and cosine ranking is performed by Koschei without requiring pgvector in the base CI image.

When the Phase 1 runtime is enabled, program IDs found in the Unified Investigation are matched against the artifact inventory before the runtime file is frozen.

## Phase 3 — Program Security Lab

Owner-only endpoint:

- `POST /api/owner/defense/lab` with `action=analyze`

The deterministic v1 analyzer builds program and instruction nodes and records conservative Solana/Anchor review surfaces:

- `KPS-S001`: UncheckedAccount without nearby CHECK rationale
- `KPS-S002`: unsafe Rust block
- `KPS-S003`: invoke_unchecked
- `KPS-S004`: remaining_accounts use
- `KPS-S005`: init_if_needed
- `KPS-S006`: realloc
- `KPS-S007`: Token-2022 permanent delegate / transfer hook / transfer fee reference
- `KPS-S008`: unwrap / expect panic surface

These are not exploit claims. Detector output uses `confidence=observed`, `lifecycle_status=hypothesis`, explicit limitations and immutable artifact evidence references.

Anchor IDL analysis builds instruction/account relationships but does not invent source-level constraints that the IDL does not contain.

## Phase 4 — Verification, Patch and Proof of Fix

Lab actions:

- `verify`: run an unpatched source bundle in the local sandbox
- `patch_propose`: ask the configured Together defense-engineer model for complete replacement files
- `patch_approve`: create an immutable human-approval record
- `patch_verify`: materialize the approved replacement files and run sandbox commands

Environment gates:

```text
KOSCHEI_DEFENSE_SANDBOX_ENABLED=false
KOSCHEI_DEFENSE_PATCH_PROPOSAL_ENABLED=false
TOGETHER_MODEL_DEFENSE_ENGINEER=
```

Allowlisted commands:

- `cargo test`
- `cargo test --workspace --all-targets`
- `cargo build-sbf`
- `anchor test --skip-local-validator`
- `trident fuzz run`

The command environment contains no wallet, keypair or RPC signing capability. Output is bounded. Paths are normalized and traversal, absolute paths and `.git` writes are rejected.

A successful approved patch verification can create an immutable proof-of-fix record. The proof remains non-authoritative for the token verdict until a future versioned deterministic acceptance rule explicitly consumes it.

## Phase 5 — Learning Flywheel

Lab actions:

- `mutate`
- `benchmark_create`
- `evaluate`
- `dataset_export`

Supported initial synthetic mutations:

- `replace_signer_with_unchecked`
- `remove_has_one_constraint`
- `remove_owner_constraint`

Synthetic artifacts are never verified and cannot be deployed through Koschei.

Benchmark evaluation compares expected, forbidden and observed deterministic rule IDs and records true positives, false positives, false negatives, precision and recall. Passing benchmark trajectories can enter the exportable training corpus as `benchmark_passed`; unreviewed candidates remain excluded.

Dataset export returns only `human_reviewed` and `benchmark_passed` examples.

## Suggested production rollout

1. Deploy migrations 066–069 with every new execution feature disabled.
2. Enable `KOSCHEI_DEFENSE_AGENT_RUNTIME_ENABLED=true` for owner-only acceptance scans.
3. Upload one verified source bundle and Anchor IDL.
4. Run deterministic analysis and inspect every finding/limitation.
5. Build a false-positive benchmark before enabling patch proposals.
6. Enable Together embeddings only after document provenance is populated.
7. Enable the sandbox only on a worker/container with strict CPU, memory, wall-time and network isolation. The web API container should remain unable to reach private keys.
8. Keep production patch application outside Koschei; merge remains a human repository action.

## Deliberate limits of this release

- No source repository is fetched automatically; the owner supplies bounded artifacts with provenance.
- No sBPF disassembler, symbolic executor, Trident harness generator or local Agave account clone is bundled yet.
- No finding becomes `reproduced` solely because static analysis or an AI narrative says so.
- No fine-tuning job is launched automatically. Koschei creates the benchmarked, provenance-backed corpus required for a later controlled training job.
