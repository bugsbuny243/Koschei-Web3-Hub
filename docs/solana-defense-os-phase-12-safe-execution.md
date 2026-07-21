# Koschei Solana Defense Intelligence OS — Phase 12

Phase 12 introduces pinned-toolchain policy and a fail-closed execution manifest for deterministic local harness execution. It does not enable stateful fuzzing and it does not grant verdict authority.

## Feature gates

```text
KOSCHEI_DEFENSE_SAFE_EXECUTION_ENABLED=false
KOSCHEI_DEFENSE_LITESVM_EXECUTION_ENABLED=false
```

Both gates must be explicitly true. Missing or malformed values are false.

## Execution boundary

The API may prepare and inspect manifests. Only the separate Defense Worker may execute them.

The worker execution profile must provide:

- no outbound network;
- no wallet, keypair, seed phrase or signing material;
- immutable source and IDL inputs;
- an empty bounded writable scratch directory;
- fixed allowlisted commands and arguments;
- fixed environment variables;
- bounded wall time, CPU, memory and output;
- one immutable result record for every attempt, including rejected attempts.

## Pinned toolchain policy

A toolchain policy is immutable and contains exact accepted values for:

- Rust compiler;
- Cargo;
- Solana CLI/runtime tooling;
- Anchor CLI;
- LiteSVM dependency or runner;
- worker image identity.

A successful `--version` command is not enough. Phase 12 requires the observed version hashes and worker image identity to match one active policy exactly.

A mismatch produces:

```text
execution_authorized=false
status=toolchain_mismatch
verdict_authority=false
```

## Owner-confirmed execution manifest

A manifest must bind:

- one immutable Phase 11 harness plan;
- the exact IDL and optional source artifact already referenced by that plan;
- program ID and network;
- one active pinned-toolchain policy;
- one fixed engine profile;
- concrete account fixtures and their hashes;
- concrete instruction arguments and their hashes;
- accepted invariants selected from or linked to the Phase 11 templates;
- deterministic seed;
- execution budgets;
- owner approval identity and timestamp.

Phase 11 templates are not automatically promoted. Missing concrete fixtures or owner-confirmed invariants keep the manifest unready.

## Initial engine

The first permitted engine is deterministic LiteSVM baseline execution.

Trident and other stateful fuzzing engines remain disabled and are reserved for Phase 13.

## Result evidence

Every attempt records immutable evidence including:

- manifest, plan, policy and worker identifiers;
- command profile and environment hash;
- fixture and argument hashes;
- deterministic seed;
- start and completion timestamps;
- exit code and termination reason;
- bounded stdout and stderr plus hashes;
- pre-state, post-state and state-delta hashes when available;
- invariant observations;
- toolchain attestation references;
- result hash;
- limitations;
- `verdict_authority=false`.

## Claim boundary

A successful Phase 12 run may establish only that a pinned deterministic harness completed for the supplied fixtures and invariants.

It does not by itself establish:

- exploitability;
- real-world reachability;
- asset impact;
- absence of vulnerabilities;
- source-to-deployment equivalence;
- proof-of-fix;
- permission to deploy a patch.

Phase 9 paired reproduction remains required for proof-of-fix. Phase 14 is required for reachability and asset-impact claims.

## First implementation slice

The first Phase 12 slice delivers:

1. immutable pinned-toolchain policies;
2. strict policy validation;
3. fail-closed matching against worker attestations;
4. immutable execution-manifest records that remain unready until all bindings and approvals are complete;
5. no source execution yet.

Execution is enabled only after this slice is merged, deployed, attested and followed by the worker sandbox slice.