# Koschei SOC — owner manual-review audit v1

**Status:** Proposed code on a stacked draft branch, not merged or deployed.

## Purpose

Create auditable timestamps for an **owner-recorded manual case review**, without claiming an authenticated analyst response, staffed SOC, contractual SLA, automatic incident action or independent on-chain evidence.

The existing owner-only incident endpoint, append-only incident audit table and no-custody ARVIS evidence boundary remain intact. This is an **owner-authored activity record**; it cannot prove the owner actually completed the investigation described in the summary.

## Request contract

On the existing authenticated `POST /api/owner/incidents/{id}` endpoint:

```json
{
  "action": "record_owner_review",
  "summary": "Manually reviewed alert provenance and checked the incident evidence links"
}
```

- Allowed only on `open`, `investigating` and `contained` incidents.
- Review summary is required (8–4000 characters). Optional unrelated `tier`, `analyst_id`, `ref`, `severity` are rejected, preventing invented proof references.
- This action is accepted only from the **existing owner authentication boundary**. No L1/L2/L3 identity, analyst access or role grant is created.
- It updates the operational `updated_at` and appends a new event. It does **not** change severity, status, linked evidence, alert evidence, ARVIS judgments, account permissions or any chain state.

## Durable event

Migration 123 restricts `security_incident_actions.action_type`; retain the constraint. Persist an existing `note` action with `actor='owner'`, the owner supplied summary and:

```json
{
  "soc_owner_review_v1": {
    "event": "owner_review_recorded",
    "scope": "manual_case_review_only"
  }
}
```

The database owns `created_at`. Multiple owner review events are allowed; history remains append-only and is available in the pre-existing `GET /api/owner/incidents/{id}` action timeline. Never infer that these events are security outcomes or action by a named analyst.

## Read-only SOC visibility

The owner-only `GET /api/owner/incidents?triage=1` projection gains optional `items[].owner_review`:

- `evidence_state = owner_manual_review_recorded`
- `actor = owner`
- `first_recorded_at`, `latest_recorded_at`
- `record_count`
- `seconds_from_case_creation` computed from the persisted case and first event timestamps.

These fields are returned only when a real, schema-valid audit event exists. **No event means no recorded owner review, not proof of absent human response.** The bounded projection (maximum 250 cases) aggregates matching audit events by persisted case ID and fails closed with HTTP 503 for an invalid or inaccessible ledger. `/owner/soc-triage` adds a read-only "Owner review record" column, rendered as safe text.

## SLA / response-time boundary

`seconds_from_case_creation` is merely the recorded elapsed time to a **manual owner-authored note**. It is not a first analyst response time, delivery acknowledgement, verified investigation completion, severity-based SLA deadline or evidence of compliance. The triage `sla_state` remains `not_configured_no_deadline_or_authenticated_assignment_claim`. No SLA policy, automatic breach detection, calendar rules, escalation dispatch or enforcement is enabled.

## Verification gates

1. Go formatting, full `go test ./...`, `go vet ./...`, exact-commit CI and security checks.
2. Postgres integration with migration 123, including no-record → two recorded owner review events → action history → rejection after resolution, only in an isolated test DB with explicit `KOSCHEI_SOC_TEST_DB_CONFIRMATION=isolated`.
3. Owner authentication and CSP regression for `/owner/soc-triage`; public/customer callers cannot see private case data or write review notes.
4. Review of UTC timestamps, clock-source consistency, concurrent cases and cache/no-store behavior.

**No deleted files, schema migration, Railway interaction, production deployment, private keys, autonomous on-chain actions, or changes to the main branch.**
