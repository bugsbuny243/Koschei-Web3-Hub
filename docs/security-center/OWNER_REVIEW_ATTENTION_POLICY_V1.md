# Koschei SOC — operational review-attention policy v1

**Delivery:** Draft stacked on PR #1475. Not merged, configured in production, or accepted against a live database.

## What this is — and is not

This is a **read-only review-attention window** calculated from persisted incident creation time and a strictly validated operator-supplied configuration. It is **not an SLA**, incident response acknowledgment, on-chain or network mitigation, staff coverage guarantee, alert delivery, breach verdict, or a regulatory claim.

No default policy is enabled. Without an explicit configuration there are **no calculated targets, deadlines or over-target cases**. The prior API's `sla_state` remains unchanged.

## Optional configuration (no deployment or environment change in this PR)

The operator may later set `KOSCHEI_SOC_REVIEW_POLICY_JSON` to a single JSON object, e.g. the following **illustrative, inactive example** (not a recommendation or a production commitment):

```json
{
  "policy_id": "soc-review-2026-01",
  "effective_at": "2026-11-01T00:00:00Z",
  "timezone": "UTC",
  "clock": "elapsed_24x7",
  "targets_seconds": {
    "critical": 900,
    "high": 3600,
    "medium": 14400,
    "low": 43200,
    "info": 86400
  }
}
```

Policy must specify all five case severities. Each target must be 60–604800 seconds (one minute to seven days); target windows must be monotonically non-decreasing from critical to info. Unknown keys, malformed JSON, non-UTC effective times, non-UTC calendar/clock settings and unsupported target values fail closed. The policy ID is a bounded opaque identifier and **not evidence of owner approval**. No secret is part of the policy.

## Read API

The existing owner-protected `GET /api/owner/incidents?triage=1` optionally adds:

- `review_policy_state`: `disabled_unconfigured` or `configured_operational_review_only`.
- `review_policy`: accepted configuration when present (non-secret).
- `items[].review_attention`: optional `policy_id`, `state`, and, **only when applicable**, `target_seconds`, `review_at`, `elapsed_seconds` and `over_target_seconds`.

Possible per-case states: `within_review_window_not_sla_guarantee`, `review_window_elapsed_not_sla_breach`, `inactive_not_measured`, `invalid_creation_time_no_review_claim`, `policy_not_yet_effective`, `case_precedes_policy_not_retroactive`, `severity_not_in_policy`.

**Business rules:**

- Only active incident states (`open`, `investigating`, `contained`) get target windows.
- Active cases created before the policy's effective timestamp are **not measured retroactively**.
- A policy whose effective timestamp is in the future is not yet applied.
- Closed or resolved cases are `inactive_not_measured`. No reconstructed historical compliance is claimed.
- Missing/future case timestamps are invalid, rather than "within SLA".
- Windows are calculated from **case creation**, not verified human acknowledgment or issue discovery. In a later case-severity change, the current severity controls the target: this v1 is not a historical policy ledger.
- Time is UTC and continuous 24/7; no weekends/holiday exclusions, pauses, assignments, escalations or calendar-aware expectations are inferred.
- Invalid operator policy returns HTTP 503 for the opt-in triage projection, never a fabricated green/empty queue.
- This does **not** generate alerts, tasks or automatic escalation; it only gives an owner-visible review hint.

The private `GET /owner/soc-triage` page shows an additional review-attention column and keeps the separate "SLA: NOT CONFIGURED" statement.

## Acceptance gates

1. Exact-head Go format, compile, unit tests, owner auth and complete CI regression.
2. Operator owner-only HTTP/API acceptance with no policy, valid policy, invalid policy and future effective date.
3. PostgreSQL acceptance with persisted active/closed cases, evidence invariants, and degraded DB. The optional integration test requires both `KOSCHEI_TEST_DATABASE_URL` and `KOSCHEI_TEST_SOC_DB_ISOLATED=1`, and only runs against a dedicated test database. An absent test DB means **not verified**, never a passing database acceptance.
4. Explicit future decision on immutable policy change audit and historical version snapshots **before any contractual SLA**.
5. Real identity/RBAC, human acknowledgment audit, alert delivery proof and on-call schedules **before** claims of staffed SOC or SLA compliance.

No database migration, file removal, service shutdown, secret, Railway change, automatic financial/on-chain action or production rollout.
