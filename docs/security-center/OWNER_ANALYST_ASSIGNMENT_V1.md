# Koschei Web3 SOC — analyst assignment v1 (additive candidate)

**Status:** Draft implementation on a stacked feature branch. Not deployed, not merged, not acceptance-verified.

## Existing architecture preserved

The existing owner-authenticated endpoints remain the only mutation boundary:

- `POST /api/owner/incidents/{id}` already accepts operator actions.
- `GET /api/owner/incidents/{id}` already returns the append-only incident timeline.
- `GET /api/owner/incidents?triage=1` (proposed in prerequisite PR #1472) provides a bounded read-only queue.

Migration `123_security_center_incident_workflow.sql` **restricts** `security_incident_actions.action_type`. This candidate intentionally **does not drop or widen** that constraint. New assignment commands are persisted as an existing, permitted `note` action, carrying a versioned `soc_assignment_v1` payload. The original case/action rows, incident state transitions, source evidence, and signed ARVIS verdicts are untouched.

## New owner commands

Call the existing authenticated `POST /api/owner/incidents/{id}` endpoint:

Assign a human-labelled analyst tier:

```json
{
  "action": "assign_analyst",
  "analyst_id": "soc-analyst-01",
  "tier": "L2",
  "summary": "Assigned for manual review"
}
```

Unassign the current analyst label:

```json
{
  "action": "unassign_analyst",
  "summary": "Rotation"
}
```

Both are allowed only while a case is `open`, `investigating`, or `contained`. `tier` must be `L1`, `L2`, or `L3`. `analyst_id` is a bounded, opaque, ASCII identifier (3–64 chars, letters/digits/dot/underscore/hyphen, starting with an alphanumeric character). No email, personal details, credential, or external identity lookup is required.

**Important:** The caller is still the authenticated owner. `analyst_id` is a manual *assignment label*, not a verified user, a login permission, proof of acknowledgment, certification, analyst staffing, or access-control grant. Actual L1/L2/L3 RBAC and account-linked acknowledgment will require a separate reviewed design.

## Durable audit event

For assignment, the append-only `security_incident_actions` table receives a `note` row with:

```json
{
  "soc_assignment_v1": {
    "status": "assigned",
    "analyst_id": "soc-analyst-01",
    "tier": "L2"
  }
}
```

For unassignment the same namespace contains `{"status":"unassigned"}`. Historical assignments remain available on the incident timeline, even after an analyst is changed or unassigned. The incident's `updated_at` advances; ARVIS evidence and verdicts do not change.

## Operator queue

The prerequisite `GET /api/owner/incidents?triage=1` projection is enriched with an optional `analyst_assignment` per returned case, derived exclusively from the latest matching persisted `note` event. Missing assignment is omitted, not fabricated. Retrieval uses the bounded case IDs and deterministic `created_at DESC, id DESC` ordering. Failure to read the assignment ledger returns HTTP 503.

The endpoint's `sla_state` **remains** `not_configured_no_deadline_or_authenticated_assignment_claim`: it makes no SLA assertions. The field will be separately versioned when an actual owner-approved SLA policy and acknowledgement events exist.

## SOC SLA policy contract — not activated

A future policy should explicitly persist its version, effective timestamp, severity-to-target mapping, event defining acknowledgment, time zone/business-hours calendar, exclusions, clock pause/resume rules, escalation owner, and audit provenance. Without these records there is no sound basis to label a case as an SLA breach. Operator workflow `contained` is also not proof of technical prevention.

## Gates before production promotion

1. Go formatting, handler tests, full repository regression, `go vet`, and exact-head CI.
2. PostgreSQL integration: assign → inspect timeline → triage → reassign → unassign → verify full historical preservation; restore test fixtures.
3. Auth verification: customer/public users cannot invoke owner-only assignment commands.
4. Workflow tests for active vs resolved/closed assignment transitions and concurrent writes.
5. UI wiring and truthful labels; deployment status and on-chain safety stay independent.

No migrations, infrastructure changes, shutdowns, deletes, auto-enforcement, key custody, or main-branch writes are part of this change.
