# Koschei Solana Defense Intelligence OS — Phase 11

Phase 11 turns an immutable Anchor IDL into a deterministic, non-executable harness plan and records which security tools are actually installed in the separate Railway Defense Worker.

## Railway web feature gate

```text
KOSCHEI_DEFENSE_HARNESS_PLANNER_ENABLED=false
```

The harness planner does not execute source and can be enabled after migration 075 is live.

## Owner endpoint

`GET|POST /api/owner/defense/harness`

### Generate a plan

```json
{
  "action": "generate",
  "idl_artifact_ref": "KDA1-ANCHOR-IDL",
  "source_artifact_ref": "KDA1-OPTIONAL-SOURCE"
}
```

The planner extracts:

- instruction names and discriminators
- nested account groups
- writable, signer, optional and PDA account metadata
- fixed addresses and declared relations
- instruction argument names and IDL types

It creates human-confirmation templates for:

- unexpected panic resistance
- signer-substitution rejection
- read-only account immutability
- allowed writable-account state transitions

These are templates, not security findings and not automatically accepted invariants.

## Why execution remains blocked

An IDL does not fully specify:

- valid initial account state
- economic conservation rules
- oracle assumptions
- valid and invalid transaction sequences
- expected failure codes
- cross-instruction business invariants

Therefore every plan has:

```text
execution_ready=false
manual_guidance_required=true
verdict_authority=false
```

LiteSVM is listed as a deterministic local-SVM candidate. Trident is listed as a stateful fuzzing candidate. Neither engine is claimed as available until the Railway worker records a successful toolchain attestation.

## Railway worker toolchain evidence

At startup the separate Railway Defense Worker runs bounded version probes for:

```text
rustc --version
cargo --version
solana --version
anchor --version
trident --version
```

The worker records immutable attestations with:

- worker ID
- tool name and command
- availability
- returned version output
- version hash
- evidence status and limitations
- attestation hash

Use:

```text
GET /api/owner/defense/harness?view=toolchains
```

to inspect the evidence.

The initial worker image includes Rust and Cargo. Solana, Anchor and Trident remain unavailable until a later pinned-toolchain image is built and accepted. Missing tools are recorded as `unavailable`; Koschei does not silently pretend that fuzzing ran.

## Persistence

Migration `075_defense_harness_toolchains.sql` adds immutable:

- Anchor harness plans
- worker toolchain attestations

A plan is bound to one IDL artifact and optionally one matching source artifact. Any change requires a new immutable plan.

## Evidence boundary

Harness planning and toolchain inventory do not establish:

- exploitability
- reachability
- asset impact
- proof-of-fix
- source-to-deployment equivalence

Those claims remain controlled by the static-analysis, deployment-verification, Railway worker and versioned reproduction phases.
