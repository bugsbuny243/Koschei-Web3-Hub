# Koschei Web3 SOC — Recorded Workflow Milestones v1

**Stage:** Stacked, review-only feature candidate; no production deployment and no SLA enforcement.

## Existing input, not another incident store

An optional read projection of the existing owner-only
`GET /api/owner/incidents?triage=1` endpoint summarizes the earliest timestamp
of these *actual persisted* `security_incident_actions`:

| Existing action | Reported timestamp | Meaning |
| --- | --- | --- |
| `investigate` | `first_investigation_at` | Owner recorded entry into investigation workflow |
| `contain` | `first_containment_record_at` | Owner changed case's operational status; **not** proof of a blocked transaction |
| `resolve` | `first_resolution_record_at` | Owner first marked case resolved; **not** cryptographic incident closure proof |
| `reopen` | `first_reopen_at` | Owner reopened case at least once |

The projection also reports the elapsed seconds from `security_incident_cases.created_at`
to the first three recorded events. The events may occur after previous reopenings:
this intentionally measures *historical first occurrence*, not current-episode SLA
or human-verified first response.

### Truth and safety rules

- Zero source events: omit `workflow_milestones`. Missing is not zero latency.
- Missing or pre-creation timestamps: omit the affected event/interval.
- Existing case status, severity, evidence references, signed verdicts, alerts,
  owner permissions, case mutations, and action history remain unchanged.
- A bounded `GROUP BY incident_id` query only aggregates the currently returned
  case IDs (at most the already-enforced 250 records).
- Database errors return HTTP 503, never a fabricated clean workflow.
- Operator action timestamps are operational records, not independent evidence
  that any analyst received the task or that technical containment succeeded.
- `sla_state` remains `not_configured_no_deadline_or_authenticated_assignment_claim`.
  The system claims no contractual deadline, breach, on-call staffing, or escalation.
- No database migrations, workers, API route additions, entitlement modifications,
  external notifications or automations are introduced.

## Acceptance gate

1. Go formatting and unit tests: `gofmt -l .`, `go test ./internal/handlers -run 'TestOwnerIncidentMilestones' -count=1`.
2. `go test ./...`, `go vet ./...`, and exact-head CI on the stacked candidate.
3. PostgreSQL 17 integration against isolated test data, checking first-action timestamps,
   missing events, reopened cases and DB unavailability.
4. Owner-only route/auth review and a UI presentation that clearly says *operator recorded*.
5. SLA policy versioning and verified analyst acknowledgement before calculating
   any SLA target, contractual success, breach or escalation.

## Dependency order

- PR #1472: bounded owner incident triage.
- PR #1473: owner-labelled analyst assignments.
- This candidate: evidence-bounded workflow milestones.

All PRs remain drafts until their own tests and reviews complete.
