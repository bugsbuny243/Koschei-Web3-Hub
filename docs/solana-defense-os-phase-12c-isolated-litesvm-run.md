# Koschei Solana Defense Intelligence OS — Phase 12C

Status: implementation contract  
Base: `main@28f1505e5943a65a960eb9c13cfaed95b7d6d8c5`  
Actor contract: `ACTOR_INVESTIGATION_ENGINE.md` v1.0  
Unified Radar ruleset: `koschei-unified-radar-rules-v1.0.0`

Phase 12C performs the first isolated deterministic LiteSVM harness command run. It consumes only evidence already accepted by Phase 12A and Phase 12B.

## Canonical investigation boundary

Applicable `ACTOR_INVESTIGATION_ENGINE.md` rules:

- §1 question 10 — missing, malformed, stale or mismatched execution evidence remains unavailable;
- §3 — toolchain, worker, materialization and runtime mismatches fail closed;
- §4 — every technical result retains exact artifact, executable and SHA-256 references;
- §5 — Defense OS execution evidence has `verdict_authority=false`.

The signed deterministic Koschei investigation verdict remains the only verdict authority. A Phase 12C run cannot change a Radar grade, verdict signature or rule result.

## Explicit scope

Phase 12C may:

- enqueue one fixed `litesvm` harness execution action;
- consume one Phase 12A execution profile with `readiness_status=ready` and `execution_allowed=true`;
- consume the exact Phase 12B immutable artifact with `artifact_role=materialized_harness`;
- run only inside the separate Defense Worker;
- re-authorize the live worker identity, image digest and executable SHA-256 immediately before launch;
- execute the immutable command policy `cargo test --locked --offline`;
- use a read-only materialized source input and one empty bounded writable scratch directory;
- enforce wall-time, output and environment limits from the immutable profile;
- persist one immutable attempt record for success, failure, timeout and rejection.

## Explicit non-scope

This phase does not:

- download, resolve, update or vendor dependencies;
- run `cargo fetch`, `cargo update` or any network fallback;
- accept caller-supplied commands or arguments;
- execute imported source in the web/API process;
- use wallet, keypair, seed phrase or signing material;
- access mainnet RPC;
- submit any transaction;
- run Trident or stateful fuzzing;
- prove exploitability, reachability, asset impact, source-to-deployment equivalence, proof-of-fix or program safety;
- apply or deploy a patch.

Every response and persisted result preserves:

```text
network_access=false
dependency_resolution=false
wallet_material_accessed=false
mainnet_rpc_accessed=false
mainnet_transaction_sent=false
verdict_authority=false
```

## Feature gates

All gates remain false by default:

```text
KOSCHEI_DEFENSE_HARNESS_EXECUTION_ENABLED=false
KOSCHEI_DEFENSE_LITESVM_EXECUTION_ENABLED=false
KOSCHEI_DEFENSE_WORKER_ENABLED=false
KOSCHEI_DEFENSE_SANDBOX_ENABLED=false
```

The API may enqueue only when both execution gates are explicitly true. The worker must refuse startup or execution when its worker/sandbox gates are not true.

No implementation PR enables these variables in production.

## Required inputs

One execution request binds exactly:

- `profile_ref` — immutable Phase 12A profile;
- `materialization_ref` — immutable Phase 12B materialization;
- `materialized_artifact_ref` — exact immutable source bundle;
- `worker_id` — must equal the profile worker;
- `worker_image_digest` — must equal the profile and live worker image;
- fixed action `run_litesvm_harness`.

The materialization must match the profile by:

- profile reference and profile hash;
- program ID and network;
- source harness artifact reference and hash;
- command policy;
- engine `litesvm`;
- Cargo manifest and lock hashes;
- materialized bundle hash.

A mismatch is persisted as a rejected attempt and no command is launched.

## Mandatory runtime authorization

Immediately before command launch, the Defense Worker must call the existing `AuthorizeHarnessExecution` boundary.

Authorization must verify:

- profile remains ready and execution-allowed;
- live worker ID matches;
- live worker image digest matches;
- required tool pin set is complete;
- each required executable resolves to the attested path;
- each executable SHA-256 still matches its pinned hash.

Any mismatch fails closed before source execution.

## Fixed command and environment

The only Phase 12C command is:

```text
cargo test --locked --offline
```

The command is represented as a fixed argv vector and is never launched through a shell.

The environment is an explicit allowlist. At minimum it must:

- set deterministic locale/time values;
- remove proxy and network configuration variables;
- point Cargo/Rust writable state only into the bounded scratch directory;
- avoid inheriting wallet, RPC, provider and deployment secrets;
- set `CARGO_NET_OFFLINE=true`;
- preserve no arbitrary caller-provided variables.

The persisted record contains an environment hash, not secret values.

## Sandbox boundary

The worker execution profile must provide:

- no outbound network at the deployment/container boundary;
- immutable read-only materialized project input;
- one newly created empty scratch directory;
- path validation preventing escape from both roots;
- bounded wall time from the immutable profile;
- bounded stdout and stderr from the immutable profile;
- process-group termination on timeout or cancellation;
- cleanup of scratch material after result persistence preparation.

Application-level `network_access=false` is evidence of configured intent, not proof that infrastructure isolation exists. The result must state the observed/configured sandbox evidence and its limitations separately.

## Immutable attempt record

Add migration `081_defense_litesvm_execution_attempts.sql`.

Each attempt is append-only and contains at least:

- attempt reference and schema version;
- request/job reference;
- profile reference and hash;
- materialization reference and hash;
- source and materialized artifact references and hashes;
- program ID and network;
- engine and fixed command policy;
- worker ID and worker image digest;
- tool attestation references;
- executable paths and SHA-256 values observed immediately before launch;
- command argv and command hash;
- environment hash;
- input bundle/Cargo manifest/Cargo lock hashes;
- configured budgets;
- start and completion timestamps;
- duration milliseconds;
- status: `rejected`, `completed`, `failed`, `timed_out` or `cancelled`;
- exit code and termination reason;
- bounded stdout/stderr and their raw SHA-256 values;
- truncation flags;
- evidence references and limitations;
- all explicit non-authority/no-mainnet flags;
- deterministic result hash;
- `verdict_authority=false`.

Rows are immutable through the existing Defense OS mutation-rejection trigger.

The result identity must not depend on database row IDs. Repeated runs are separate immutable attempts, but identical deterministic outputs must produce the same evidence/result hash when timestamps and attempt identity are excluded from that hash payload.

## Worker queue integration

Do not weaken the existing `verify_bundle` worker action.

Add a separate action:

```text
run_litesvm_harness
```

The new action accepts only profile/materialization references. It accepts no command list, replacement files, finding reference or patch reference.

The queue request hash is deterministic over immutable input references and the fixed action. Idempotency must prevent accidental duplicate active jobs for the same profile/materialization pair while allowing a deliberate later rerun after a terminal state.

The web/API process may enqueue and read status only. `ProcessWorkerJob` launches the command only in the separate Defense Worker process.

## Owner API

Add owner-only:

```text
GET|POST /api/owner/defense/litesvm-execution
```

POST body:

```json
{
  "action": "enqueue",
  "profile_ref": "KHEP1-...",
  "materialization_ref": "KHM1-..."
}
```

GET supports bounded lookup/listing by attempt, job, profile and materialization references.

The endpoint:

- requires owner session and database;
- is disabled unless both Phase 12C execution gates are true;
- never executes synchronously;
- never accepts a command or environment payload;
- returns explicit no-mainnet/no-verdict-authority flags.

## Expected file ownership

Implementation should remain bounded primarily to:

- `koschei/api/migrations/081_defense_litesvm_execution_attempts.sql`;
- `koschei/api/internal/defense/litesvm_execution.go`;
- `koschei/api/internal/defense/litesvm_execution_test.go`;
- `koschei/api/internal/defense/worker.go` and focused tests;
- `koschei/api/cmd/defense-worker/main.go` only for live image/gate wiring if required;
- `koschei/api/internal/handlers/defense_litesvm_execution.go` and tests;
- `koschei/api/internal/http/defense_os_routes.go` and route inventory tests;
- `.env.example` and Defense OS documentation;
- required CI path/build coverage only if not already present.

Do not refactor unrelated ARVIS, auth, entitlement, alert, Radar, transaction-guard or legacy verification paths.

## Fail-closed error taxonomy

Use stable internal error/status codes for at least:

- `execution_gate_disabled`;
- `profile_blocked`;
- `materialization_not_found`;
- `materialization_profile_mismatch`;
- `materialized_artifact_mismatch`;
- `worker_identity_mismatch`;
- `worker_image_mismatch`;
- `tool_pin_mismatch`;
- `sandbox_unavailable`;
- `dependency_unavailable_offline`;
- `execution_timeout`;
- `output_limit_reached`;
- `process_failed`;
- `execution_cancelled`.

Provider, environment or sandbox failure is never reported as a program finding.

## Required validation

Before merge:

1. PostgreSQL 17 migration chain through migration 081;
2. immutable execution-attempt mutation rejection;
3. ready profile + matching materialization accepted;
4. blocked profile rejected without command launch;
5. mismatched profile/materialization/artifact rejected;
6. stale worker image rejected;
7. executable path or SHA-256 mutation rejected immediately before launch;
8. caller command/environment injection impossible;
9. shell metacharacters remain inert because no shell is used;
10. offline dependency absence produces `dependency_unavailable_offline`, not network access;
11. timeout kills the process group and persists a timed-out attempt;
12. stdout/stderr bounds and truncation hashes are deterministic;
13. success and non-zero exit records are immutable;
14. duplicate active enqueue is idempotent;
15. worker lease recovery cannot execute one attempt concurrently twice;
16. API process test proves it cannot launch source;
17. complete `go test ./...`;
18. `go vet ./...`;
19. Linux builds for API, Defense Worker and Defense Sentinel;
20. public JavaScript and Turkish-copy checks when touched;
21. gitleaks;
22. govulncheck;
23. high-severity/high-confidence gosec gate.

## Completion rule

Phase 12C is complete only after:

- code, migration, tests and documentation are merged;
- all required CI gates pass;
- migration 081 is deployed successfully;
- all execution gates remain off during deployment validation;
- one owner-approved reference harness is run on the isolated worker;
- the second run produces the same deterministic evidence/result hash for the same materialized inputs, excluding attempt identity and timestamps;
- no mainnet, wallet or verdict-authority boundary is weakened.
