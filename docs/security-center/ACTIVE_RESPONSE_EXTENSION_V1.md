# Koschei-Web3-Hub — Active Response Extension (v1, shadow-only)

## Goal and compatibility

This is an **additive defense planning layer**, not a replacement for the
existing Defense OS, SOC/incident workflows, Transaction Guard, HTTP API,
authentication, entitlement, workers, migrations, or dashboards.

- Existing routes, handlers, database schema, policy decisions, and
  feature-flag behavior are **unchanged**.
- No new dependency, background loop, outbound HTTP request, shell execution,
  privileged service account, key or provider credential is introduced.
- No external operation is triggered. The code only constructs a typed plan.
- Existing `DefensivePolicy`, `SecurityFinding`,
  `DefensiveActionRequest`, `ValidateFinding` and
  `EvaluateDefensivePolicy` remain authoritative; no breaking wire changes.
- This extension is not wired into the request router or production workers.

The objective is to create the validated, non-executing contract that can later
be connected to explicitly authorized, measured provider adapters without
risking a regression in the running product.

## Action catalog

| Domain | Action | Managed inventory target | Proposed effect |
|---|---|---|---|
| SOAR / XDR | `isolate_workload` | `workload` | isolated |
| SOAR / XDR | `terminate_session` | `session` | revoked |
| SOAR / XDR | `revoke_api_key` | `api_key` | revoked |
| Zero Trust | `isolate_segment` | `network_segment` | isolated |
| Network / DDoS | `bgp_blackhole` | `protected_prefix` | blackholed |
| Network / DDoS | `ddos_scrubbing` | `protected_prefix` | scrubbing_enabled |
| Web3 | `pause_contract` | `smart_contract` | paused |
| Web3 | `block_transaction` | `transaction_policy` | blocked_before_signing |
| Hardware | `air_gap` | `hardware_link` | disconnected |
| Hardware | `lock_hsm` | `hsm_key` | signing_disabled |

These are **capability identifiers**, not statements that a provider has been
integrated. In particular, an external service cannot pause or freeze an
arbitrary immutable contract without an existing authorized on-chain control.
Transaction blocking only applies to transaction submission paths the operator
actually controls. Network blackholing is highly disruptive and must be
restricted to operator-owned and approved prefixes.

A maximum intended duration of zero means the proposed action has no automatic
TTL contract at this phase; it **does not** imply an automatically recoverable
or safe permanent change. For actions that can be reversed, a future provider
adapter must supply its own verified rollback and cleanup path.

## Safety and provenance guarantees

`ProposeActiveResponse` requires:

1. A non-empty action, tenant, target, and action ID, and an action from the
   fixed catalog (no arbitrary command execution).
2. A trusted finding and known evidence set; all action evidence IDs must match
   the verified finding, independent of ordering.
3. Exact tenant and target matching with a **trusted managed-asset inventory**.
   Unmanaged assets, control-plane assets, cross-tenant targets and incorrect
   target types are denied.
4. Existing explicit per-tenant, per-action and per-target policy allowlists.
5. Minimum confidence of 0.85 for ordinary actions and 0.95 plus high/critical
   severity for high-impact actions.
6. Mandatory human-approval flag for high-impact actions or production-critical
   targets; existing `HumanGateActions` still applies.

The inventory and evidence set **must** be loaded by trusted back-end services.
Accepting a client's assertion of ownership, policy, or known evidence is not
an authorization mechanism. A plan by itself is never an execution permit.

Every returned plan has:

- `execution_performed=false`
- `effect_agreement=not_exercised`
- `status=shadow_proposed` or `denied`

No receipt claims observed effects or verified quarantine in this version.

## Required gates before provider execution (not implemented in v1)

Before enabling any action with side effects, add and independently verify:

- Trusted provider adapters with least-privilege credentials and per-tenant
  resource binding. Do **not** expose generic HTTP, shell, router CLI, RPC,
  or arbitrary smart-contract method execution as an action.
- Explicit opt-in per integration, read-only discovery and dry-run by default.
- Verified delegated authority / operator ownership, including BGP prefix
  authorization and contract-specific pause roles or multisig policies.
- Authenticated approval records, separation of duties for disruptive actions,
  bound to immutable request hashes and short expiry times.
- Durable idempotency keys, replay prevention, concurrency controls and an
  incident audit journal; do not trust memory-only locks in a scaled deployment.
- Rate limits, blast-radius and TTL constraints (including management-network
  break-glass protection), and per-provider recovery procedures.
- Timeouts, bounded retries with backoff, deduplicated state transitions and
  provider response attestation.
- Independent effect observation and confirmation via the existing
  `BuildDefensiveReceipt` contract. Unknown effect or partial coverage must
  remain `indeterminate`, never `agree`.
- Safe shadow/canary/production rollout with explicit rollback criteria and
  regression acceptance gates.

No absolute "millisecond stop" SLA can be asserted without measuring the
actual end-to-end provider and network propagation times. BGP, multi-signature
on-chain changes, HSM operation, and physical relay actions can take materially
longer than in-process decision evaluation.

## Test and review notes

Run from `koschei/api` with the repository's pinned Go toolchain:

```bash
go test ./internal/defense -run 'TestActiveResponse' -count=1
go test ./...
go vet ./...
```

The extension tests cover the five domains, no execution, cross-tenant
restrictions, unmanaged/control-plane targets, incorrect target kinds,
tampered evidence, unknown actions, empty target IDs, low-confidence and
low-severity high-impact findings, and preservation of existing human gates.

Do not claim full-repository acceptance before the CI matrix on this branch
completes. The historical uploaded dead-code list is not a mandate to delete
legacy functions or integrations; this PR intentionally removes none.


## v1.1 offline approval and provider binding review (PR #1506)

An additive file, `internal/defense/active_response_review.go`, extends the
shadow-only flow with a local verification layer. It does not register a route,
grant an execution permit, call a provider, or mutate existing database tables.

### Review requirements

- Re-run the original policy/evidence/asset planner and compare the complete
  shadow plan against the verified result before evaluating any approvals.
- Hash the exact defensive policy, trusted finding, request (including action
  parameters), managed asset and shadow plan into a domain-separated SHA-256
  review digest. **The digest is not an authorization token.**
- Require an enabled provider binding with an exact matching tenant, managed
  target, action and capability. These declarations MUST be obtained from a
  trusted registry and are not provider-side proof of ownership.
- Use Ed25519 signatures from pre-enrolled, server-side trusted approver public
  keys; a request must not supply its own trust anchors.
- Bind each signature to the plan digest, signer identity, issuance and
  expiration; maximum approval validity is five minutes.
- Require at least two distinct signing-key IDs for high-impact actions and
  one for other actions if the existing policy/asset requires human approval.
  A deployment must additionally verify those key IDs belong to separate
  independently authenticated human operators; two keys alone are not proof
  of two different people.
- Reject altered signatures, expired approvals, duplicate signer IDs, disabled
  or cross-tenant bindings and mutated plans.

The only successful status is `ready_for_manual_integration`. The resulting
`execution_performed` field is **always false**. No live action is exposed.
A durable action ledger, trusted approver identities, replay prevention,
provider-authority attestation, rollback, independent observed-effect
evidence and authorization to execute remain deployment prerequisites.

### Validation

```bash
cd koschei/api
go test ./internal/defense -run 'TestActiveResponse' -count=1
gofmt -l internal/defense/active_response*.go
```

This is an additional offline admission check, not an operational SLA, a
functional DDoS blackholing implementation, an on-chain pause authority, or
a provider integration.

## v1.2 recovery readiness and offline witness envelopes

Two independent modules extend **review only**, without registering any
network, router, HSM, smart-contract, identity, cloud or quarantine executor:

- `active_response_recovery.go`: evaluates provider- and target-bound
  recovery prerequisites. Network isolation/blackhole/scrubbing plans require
  a bounded provider-enforced TTL and a rollback test attested within the
  previous 30 days. Every action requires a trusted runbook, preserved
  management-network access, verified break-glass procedure and a recovery
  owner distinct from the listed observer.
- `active_response_witness.go`: validates a short-lived Ed25519-signed
  offline observation envelope against a trusted observer key registry, action
  plan digest, tenant, target, provider, evidence SHA-256 digest, observer
  identity and coverage metadata. Matching, conflicting and incomplete
  witness reports are different statuses.

The key distinction: **an offline witness statement does not prove that
an action was executed**. In every witness assessment,
`execution_performed=false` and `effect_agreement=not_exercised`
regardless of whether the reported state matches the plan. These
witnesses are not fed into `BuildDefensiveReceipt` because that would
risk implying observed provider effects with no trusted execution record.

The staging readiness statuses are `ready_for_staging_review` for
reversible network controls and `manual_recovery_review` for actions whose
automatic reversal cannot be assumed (including key revocation, contract
pause, physical disconnect and HSM lock).

**Trust boundary:** This code does not itself establish that provider
inventory, rollback attestations, operator/observer IDs, coverage reports or
signed observer claims reflect real infrastructure state. The caller must
load all trusted inputs from an operator-managed registry, not a public
request body. Signed assertions by distinct IDs are not proof of separate
human identities; that remains a separate enrollment requirement.

For future live operation, implement a durable signed execution-result
ledger and independent external state confirmation before producing any
receipt that claims `agree`. Keep unknown scope and incomplete evidence
indeterminate, not successful. None of those live capabilities are
introduced by this PR.

### Validation additions

```bash
cd koschei/api
go test ./internal/defense -run 'TestActiveResponse' -count=1
gofmt -l internal/defense/active_response*.go
```

The new tests include recovery TTL, break-glass, tested rollback recency,
separate recovery and observer IDs, cross-tenant bindings, signed witness
tampering, freshness, coverage gaps, disagreement and prevention of false
real-world effect claims.

## v1.3 opt-in PostgreSQL shadow ledger (not automatically installed)

`active_response_ledger.go` implements an **unwired, explicit-call-only**
database repository for offline action-review records. The SQL schema lives at
`docs/security-center/ACTIVE_RESPONSE_LEDGER_STAGING_V1.sql` rather than the
production migration directory; therefore routine app startup and CI migration
runners do not install or invoke it.

The staging database installation must be explicitly reviewed by an
administrator. After installation the opt-in repository:

1. Requires an offline, unexecuted shadow plan, completed review and complete
   recovery preflight. These are **not** authentication credentials; the
   caller must obtain them from verified, trusted server-side sources.
2. Opens a PostgreSQL serializable transaction, acquires a per-tenant
   transaction-scoped advisory lock and checks the existing action ID.
3. Returns `shadow_duplicate` for an exact re-submit without creating an
   additional audit event; rejects action-ID collisions with different target,
   provider or plan digest.
4. Appends a `shadow_recorded` action and a SHA-256-linked audit entry
   atomically. Both tables have tenant/action uniqueness; the SQL script
   provides immutable-row triggers against UPDATE/DELETE operations.
5. Fails closed on missing DB storage, malformed identifiers, invalid
   review/recovery state or invalid chain hash. No provider command or
   network operation is implemented.

**Operational limits:** The hash chain depends on a trusted database, correct
transaction isolation, append-only privileges and external periodic
checkpoints. A database owner can disable triggers or delete the entire
database; hashes alone do not make the ledger tamper-proof. Audit ordering is
per tenant and must be independently reconciled. A duplicate result does
not prove the caller's credentials or approvals are currently authorized.
The adapter performs **audit persistence**, not anti-replay authorization for
future live actions; a separate single-use execution claim ledger and
idempotent provider adapters are still required before production use.

Testing: `go test ./internal/defense -run 'TestActiveResponse' -count=1`
exercises offline ledger entry hashing, invalid state rejection and no-storage
fail-closed behavior. PostgreSQL-specific transaction, concurrency, schema
permissions and outage/failover acceptance tests must run in a dedicated
staging environment before this code is used to store real security events.

## v1.4 independently anchored ledger verification and verified ingress

Added `active_response_ledger_integrity.go` and
`active_response_ledger_verified.go` as **offline, non-routed** modules.

- `VerifyShadowLedgerSnapshot` checks an ordered, single-tenant audit
  interval against **separately trusted** checkpoint root and head hashes.
  It recomputes each v1 SHA-256 chain digest, checks link continuity,
  rejects duplicate action IDs and chain hashes, rejects tampered scope,
  timestamps and content, and never reports live execution. The caller must
  load records in canonical database sequence order.
- This check **does not** establish authenticity if the expected checkpoint
  hashes come from the same potentially modified database. Publish and
  protect periodic checkpoint roots/heads independently. A verified subrange
  does not prove the absence of records before or after the anchored range.
- `PrepareVerifiedShadowLedger` recomputes the original action policy,
  known-evidence matches, managed asset and provider binding, signed offline
  human approvals, content-bound review digest and recovery preflight. It
  rejects fabricated `ready` statuses instead of trusting caller-provided
  review results. The surrounding service must still load every trust
  anchor and asset record from authoritative, authenticated sources.
- `RecordVerifiedShadowLedger` is the preferred optional staging write
  entry point and invokes the existing shadow ledger only after this
  recomputation. It is not wired to production routes, workers or startup.
- Dual human approvals additionally require **distinct Ed25519 public keys**,
  not only distinct registry IDs. Separate real-world operator identities
  still require authoritative enrollment outside this module.

### Acceptance and limitations

The new tests exercise valid anchored snapshots, tampered payloads, altered
timestamps, changed roots and heads, duplicate/disconnected events,
foreign-tenant records, invalid or shared signing keys, untrusted evidence,
disabled or misbound provider claims, invalid recovery attestations and
storage-unavailable behavior. This does **not** replace a PostgreSQL
integration test under concurrent writers, DB restarts and expired signing
credentials. No live anti-replay execution ledger or provider effect has been
implemented or enabled.

## v1.5 signed checkpoints and safe staging write boundary

`active_response_ledger_checkpoint.go` validates offline, externally
retained Ed25519-signed checkpoint manifests that bind a tenant, chain
root, chain head, record count, signer identity and UTC issuance timestamp.
Signatures are checked against **server-supplied, independently managed**
public keys. The verifier then checks each chain entry against both
checkpoint hashes; changed rows, missing entries, mismatched tenants,
unknown signers, future timestamps and forged signatures are rejected.

This module **does not sign or publish checkpoints**, install keys, fetch
ledger rows or certify the absence of missing *other* checkpoints. The
checkpoint must be stored separately from the database, with controls
against checkpoint deletion, substitution and rollback. A valid signature
authenticates the declared snapshot, not a live provider action.

The low-level `recordShadowLedger` database writer is **package-private**;
only `RecordVerifiedShadowLedger` is an exported staging write entry point.
It recomputes policy, trusted evidence, managed asset binding, Ed25519
approvals and recovery prerequisites before calling the low-level writer.
No route calls this function. Even this recomputation depends on trusted
back-end loaders and must never receive caller-selected trust anchors.

The additional tests assert checkpoint integrity, signature authenticity,
record-count consistency, tenant boundaries, future-time rejection and
low-level write fail-closed behavior. A **real PostgreSQL staging acceptance
run**, independently protected checkpoint storage and a durable replay
claim for any future live executor remain outstanding.

## v1.6 explicitly isolated PostgreSQL staging acceptance

`active_response_ledger_postgres_test.go` adds an opt-in PostgreSQL
integration test. The test **skips automatically** unless the dedicated
`KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL` variable is set, then
queries `current_database()` and **refuses every name other than**
`koschei_shadow_test`. It does not apply migrations or create tables.
The staging SQL file must already have been reviewed and installed by a
database administrator in this disposable, isolated test database.

In an authorized staging environment only:

```bash
# Create an isolated database named koschei_shadow_test beforehand.
# Review and apply docs/security-center/ACTIVE_RESPONSE_LEDGER_STAGING_V1.sql
# with your staging DBA; do not run it on production.
cd koschei/api
KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL='<dedicated-test-DSN>' \
  go test ./internal/defense -run '^TestActiveResponsePostgresStagingLedgerAcceptance$' -count=1
```

The test checks the initial append, identical action-ID replay, rejection of a
different digest under the same action ID, the one-plan/one-audit-row invariant
and chain re-verification using data retrieved from PostgreSQL. It writes
uniquely named tenant-scoped test records and does not clean up append-only
rows; dispose of the dedicated test database after acceptance.

**Not yet demonstrated:** simultaneous write races, serializable retry
behavior, DB failover, role-level permissions and independent off-database
checkpoint archival. Do not equate this opt-in staging test with a completed
production acceptance.


### PostgreSQL timestamp precision integrity

The shadow ledger writer normalizes `recorded_at` to UTC
**microsecond precision** before calculating its SHA-256 chain hash,
matching PostgreSQL `timestamptz` storage precision. An added
regression test checks that encoding a timestamp, round-tripping it
through microsecond storage precision and hashing it again produces
the same digest. Without this, nanosecond-level Go timestamps could
produce false audit-integrity failures after database reads.


## v1.7 isolated PostgreSQL concurrency and fail-closed checks

The offline staging ledger now retries PostgreSQL SQLSTATE `40001`
serialization failures for **at most five fresh transactions**, stopping on
context cancellation. Uniqueness (`23505`), permission (`42501`),
connection (`08006`) and ambiguous commit errors are **never retried**
automatically, to avoid hiding conflicts or uncertain write outcomes.
No side-effecting provider action is reachable from this retry path.

New ordinary unit tests cover successful serialization retries, exhausted
retries, cancellation before and during an attempt, terminal SQLSTATEs and
a closed database handle. These run without database access.

The additional opt-in staging test
`TestActiveResponsePostgresStagingLedgerConcurrency` uses the same strict
`current_database() = 'koschei_shadow_test'` guard and checks:

- parallel distinct action IDs within one tenant, including serializable
  retry handling;
- concurrent duplicate requests: one inserted audit row and the rest
  reported as duplicates;
- an ordered SQL read-back of all resulting ledger rows and SHA-256 chain
  verification.

Run both staging acceptance tests **only** against the isolated disposable
database provisioned manually with the staging SQL script:

```bash
cd koschei/api
KOSCHEI_ACTIVE_RESPONSE_STAGING_TEST_DATABASE_URL='<dedicated-test-DSN>' \
  go test ./internal/defense -run '^TestActiveResponsePostgresStagingLedger' -count=1 -v
```

The isolated PostgreSQL acceptance checks have been **implemented but not
executed in this work session**. Production DB failover, operator privilege
separation, independent long-term checkpoint storage and durable live
execution replay claims remain unverified. CI without that variable simply
skips these database-writing staging tests.

## v1.8 least-privilege PostgreSQL acceptance

The isolated GitHub Actions PostgreSQL 17 job additionally provisions a
disposable `koschei_shadow_ci_writer` account with **SELECT and INSERT only**
on the two shadow ledger tables, plus the necessary sequence access. The job
explicitly confirms that this account has **neither UPDATE nor DELETE**
permission. The complete sequential and concurrency acceptance tests run
under that limited account as well as under the staging database owner.

The low-level shadow writer now uses ordinary SELECT for duplicate checking:
`SELECT FOR UPDATE` was unnecessary because a tenant-scoped PostgreSQL
advisory transaction lock already serializes shadow appends, and it
incorrectly required UPDATE privilege on append-only records. Serializable
transactions still handle snapshot conflicts through the bounded
SQLSTATE `40001` retry path.

The test principal exists only inside the ephemeral CI PostgreSQL service.
Its fixed password is **CI-only test data**, not a real service credential.
A production-grade installation must instead provision unique short-lived
credentials in its operator-controlled secrets service, enforce restricted
network access and PostgreSQL role membership, and independently review
schema ownership and protected checkpoint storage. No such production
installation is performed or authorized by this PR.

## v1.9 read-only reconciliation after ambiguous storage outcomes

`InspectShadowLedgerReceipt` adds a side-effect-free query for a single
tenant/action on the configured PostgreSQL database. Its trusted caller must
supply the expected tenant, target, provider and plan digest from authoritative
records. A single `FULL OUTER JOIN` observes both shadow plan and audit rows,
rejects partial/mismatched records, and recomputes the content-bound SHA-256
hash for a complete record.

The result states are deliberately conservative:

- `shadow_receipt_consistent`: a locally consistent plan and audit receipt
  was found under the expected scope and digest. It proves **neither** a
  provider effect **nor** the entire historical chain without a separately
  verified checkpoint.
- `shadow_receipt_absent_unconfirmed`: the consulted data source returned
  no matching row. This is **not** proof of a failed commit, nor permission to
  resend the action, especially if the source is a lagging replica.
- `shadow_receipt_unverified` with an error: failed query, canceled request,
  malformed scope, partial row, conflicting digest or hash corruption.
  Fail closed. There is no automatic retry of ambiguous writes.

The opt-in PostgreSQL acceptance suite checks consistent, absent and
conflicting receipts. A further disposable **backend session-termination**
test records a shadow action, terminates an owned test session with
`pg_terminate_backend`, then checks a new connection can still reconcile
the already committed receipt. This is **not** a database host outage,
failover, crash during COMMIT, or distributed durability test. Those cases
remain separate acceptance requirements. The test DSN is restricted to
loopback `127.0.0.1:5432/koschei_shadow_test`, so the session termination
test cannot intentionally target a remote service.

No production connection, API route, worker, transaction dispatch or
provider adapter uses this reader. Storage receipt reconciliation is
read-only and does not authorize live defensive actions.
