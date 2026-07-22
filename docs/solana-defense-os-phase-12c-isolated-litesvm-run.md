# Koschei Solana Defense Intelligence OS — Phase 12C

Status: implementation in progress  
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
KOSCHEI_DEFENSE_NETWORK_ISOLATED=false
```

The API may enqueue only when both execution gates and the existing worker-queue gate are explicitly true. The separate worker requires its worker/sandbox gates and must also require explicit no-egress deployment evidence through `KOSCHEI_DEFENSE_NETWORK_ISOLATED=true` before accepting the Phase 12C action.

The flag records operator-confirmed configuration only. It is not accepted as proof of infrastructure isolation by itself, and every attempt retains that limitation.

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

A mismatch is rejected and no command is launched. Once enough immutable identity is available, a rejected launch attempt is persisted rather than being promoted into a program finding.

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

The environment is an explicit allowlist. It:

- fixes locale, timezone, terminal and source-date values;
- removes proxy, provider, RPC, deployment and wallet variables by not inheriting the parent environment;
- points writable Cargo/Rust/target/temp state only into the bounded scratch directory;
- binds the persisted environment hash to the exact template, pinned tool paths and fixed settings;
- sets `CARGO_NET_OFFLINE=true`;
- preserves no arbitrary caller-provided variables.

The persisted record contains the environment hash, not secret values.

A worker image that lacks the dependencies required by the immutable lock file may fail with `dependency_unavailable_offline`. This failure is explicit evidence of unavailable offline dependencies; it never triggers a network fallback. A successful production reference run therefore requires a separately reviewed immutable dependency-cache or equivalent image-baked dependency policy before the execution gates are enabled.

## Sandbox boundary

The worker execution profile provides:

- no outbound network at the deployment/container boundary;
- immutable read-only materialized project input;
- one newly created empty scratch directory;
- path validation preventing escape from both roots;
- bounded wall time from the immutable profile;
- bounded stdout and stderr from the immutable profile;
- process-group termination on timeout or cancellation;
- cleanup of input and scratch material after result preparation.

Application-level `network_access=false` and `KOSCHEI_DEFENSE_NETWORK_ISOLATED=true` are evidence of configured intent, not proof that infrastructure isolation exists. The result states the configured sandbox evidence and its limitations separately.

## Immutable attempt record

Migration `081_defense_litesvm_execution_attempts.sql` adds the append-only attempt store and a partial unique index preventing duplicate active jobs for one immutable request hash.

Each attempt contains at least:

- attempt reference and schema version;
- request/job reference and attempt number;
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

The result identity does not depend on database row IDs. Repeated runs are separate immutable attempts, but identical deterministic outputs produce the same evidence/result hash when timestamps, duration and attempt identity are excluded from the hash payload.

## Worker queue integration

The existing `verify_bundle` worker action remains unchanged in authority and behavior.

Phase 12C adds a separate action:

```text
run_litesvm_harness
```

The new action accepts only profile/materialization references. It accepts no command list, replacement files, finding reference or patch reference. The queue replaces no caller field; it binds the fixed command internally after the request passes validation.

The request hash is deterministic over immutable input references and the fixed action. A PostgreSQL partial unique index prevents accidental duplicate queued/running jobs for the same request while allowing a deliberate later rerun after a terminal state.

The web/API process may enqueue and read status only. The only command-launch path is `ExecuteLiteSVMWorkerJob` called through the separate Defense Worker runtime.

## Owner API

Owner-only:

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

The request decoder rejects unknown fields and multiple JSON values. This makes command, argument and environment injection impossible through the dedicated endpoint.

GET supports bounded lookup/listing by attempt, job, profile and materialization references.

The endpoint:

- requires owner session and database;
- is disabled unless the Phase 12C execution gates and worker queue gate are true;
- never executes synchronously;
- never accepts a command or environment payload;
- returns explicit no-mainnet/no-verdict-authority flags.

## Current implementation slice

Implemented on the Phase 12C draft branch:

- migration 081 immutable attempt schema;
- deterministic active-job idempotency;
- fixed-action queue validation;
- exact profile/materialization/artifact/file/Cargo hash preparation checks;
- immediate live worker/tool re-authorization through the Phase 12A boundary;
- direct pinned-Cargo argv launch in the separate worker;
- explicit allowlisted environment;
- read-only input and bounded scratch directories;
- process-group cancellation/timeout handling;
- bounded stdout/stderr and deterministic hashes;
- immutable result persistence and repeatable result hash;
- dedicated owner control-plane endpoint;
- default-off web and worker gates;
- focused fail-closed tests.

The branch remains draft until the complete required test/security/build gates pass and the immutable dependency-cache policy needed for the owner-approved reference success case is settled.

## Expected file ownership

Implementation remains bounded primarily to:

- `koschei/api/migrations/081_defense_litesvm_execution_attempts.sql`;
- `koschei/api/internal/defense/litesvm_execution.go`;
- `koschei/api/internal/defense/litesvm_runner.go`;
- focused Defense tests;
- `koschei/api/internal/defense/worker.go`;
- `koschei/api/cmd/defense-worker/main.go`;
- `koschei/api/internal/handlers/defense_litesvm_execution.go`;
- `koschei/api/internal/http/defense_routes.go`;
- Defense OS environment and phase documentation.

Unrelated ARVIS, auth, entitlement, alert, Radar, transaction-guard and legacy verification paths are not refactored.

## Fail-closed error taxonomy

Stable result/termination states include:

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
- one owner-approved reference harness runs on the isolated worker with an immutable reviewed dependency policy;
- the second run produces the same deterministic evidence/result hash for the same materialized inputs, excluding attempt identity, timestamps and duration;
- no mainnet, wallet or verdict-authority boundary is weakened.
