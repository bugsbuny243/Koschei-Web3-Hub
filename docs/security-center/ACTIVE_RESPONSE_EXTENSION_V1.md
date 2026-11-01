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
