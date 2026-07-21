# Koschei Solana Defense Intelligence OS — Phase 12A

Phase 12A introduces an immutable, fail-closed authorization boundary between a Phase 11 harness plan and any future isolated execution.

It does **not** execute a harness. It makes it impossible for a later worker action to claim authorization without exact source, invariant and toolchain evidence.

## Canonical investigation contract

Applicable `ACTOR_INVESTIGATION_ENGINE.md` sections:

- §1 question 10 — verified, observed and unknown states stay distinct;
- §3 — missing or unpinned evidence remains unavailable;
- §4 — technical claims retain exact evidence references;
- §5 — Defense evidence cannot alter deterministic verdict authority.

Actor ruleset: `v1.0`  
Unified Radar ruleset: `koschei-unified-radar-rules-v1.0.0`

## Web feature gate

```text
KOSCHEI_DEFENSE_HARNESS_EXECUTION_GATE_ENABLED=false
```

The gate is owner-only and disabled by default.

## Worker image identity

The isolated Defense Worker must receive an immutable image identity:

```text
KOSCHEI_DEFENSE_WORKER_IMAGE_DIGEST=sha256:<64 lowercase hex>
```

The value must identify the deployed worker image. A deployment ID, mutable tag or branch name is not accepted as a substitute.

Every startup toolchain probe records:

- worker ID;
- worker image digest;
- tool name and exact version output;
- resolved executable path;
- executable SHA-256;
- version-output SHA-256;
- immutable attestation hash;
- limitations and `verdict_authority=false`.

A command may be available but still **unpinned** when the image digest or executable hash is absent. Availability alone cannot authorize execution.

## Harness artifact contract

A future run cannot execute the target source bundle directly. It requires a separate immutable `source_bundle` artifact whose metadata contains:

```json
{
  "artifact_role": "harness",
  "harness_plan_ref": "KHP1-..."
}
```

The harness artifact must match the plan's program ID and network. It remains review material; its presence does not prove correctness.

## Owner endpoint

```text
GET|POST /api/owner/defense/harness-execution
```

### Assess and lock an execution profile

```json
{
  "action": "assess",
  "plan_ref": "KHP1-...",
  "harness_artifact_ref": "KDA1-...",
  "engine": "litesvm",
  "worker_id": "defense-worker-1",
  "worker_image_digest": "sha256:...",
  "confirmed_invariants": [
    {
      "template_id": "KHT-NO-PANIC:withdraw",
      "statement": "The confirmed grammar must return a program error and must not panic."
    }
  ],
  "max_duration_seconds": 120,
  "max_output_bytes": 262144
}
```

Supported Phase 12A engines:

- `litesvm` — requires pinned `rustc` and `cargo` attestations;
- `trident` — requires pinned `rustc`, `cargo`, `solana`, `anchor` and `trident` attestations.

A profile becomes `ready` only when:

1. the Phase 11 plan exists and references target source;
2. the separate immutable harness artifact matches the plan;
3. every confirmed invariant references a real template in the plan;
4. every required tool is available and has exact executable and version hashes;
5. every tool attestation matches the requested worker image digest;
6. resource ceilings are valid;
7. the deterministic command policy is fixed by Koschei, not supplied by the caller.

Otherwise the profile is persisted as `blocked` with explicit limitations. A blocked profile never becomes ready through mutation; a new immutable profile must be created after evidence changes.

## Fixed command policy

Phase 12A records policy only. It does not launch these commands.

LiteSVM profile:

```text
cargo test --locked --offline
```

Trident profile:

```text
trident fuzz run
```

Both policies require:

```text
network_access=false
wallet_keys=false
mainnet_rpc=false
mainnet_transaction_sent=false
```

The future Phase 12C worker action must use the exact stored policy and must not accept an arbitrary command string.

## Persistence

Migration `078_defense_harness_execution_gate.sql` adds:

- binary and worker-image pin fields to toolchain attestations;
- immutable `defense_harness_execution_profiles` records;
- indexes for plan and worker lookups;
- update/delete rejection through the existing Defense OS immutable trigger.

## Explicit non-claims

Phase 12A does not establish:

- that a harness compiled;
- that LiteSVM or Trident ran;
- exploitability or reachability;
- asset impact;
- proof-of-fix;
- source-to-deployment equivalence;
- program safety.

Until Phase 12B and Phase 12C are implemented, every API response preserves:

```text
harness_executed=false
mainnet_transaction_sent=false
verdict_authority=false
```
