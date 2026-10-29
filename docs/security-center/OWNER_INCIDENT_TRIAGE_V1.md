# Security Center — owner incident triage projection v1

**Status:** Additive, owner-only, read-only candidate; not deployed or acceptance-certified.

## Endpoint

`GET /api/owner/incidents?triage=1&limit=100&status=<optional-status>`

Uses existing owner authentication and persisted `security_incident_cases`. No new route or database migration. The default `GET /api/owner/incidents` and all mutation endpoints remain unchanged.

## Response interpretation

- `items[].incident` preserves the original persisted incident record and its evidence, alert, and dossier references.
- `age_seconds` is wall-clock age since creation, **not** an SLA measurement or time-to-compromise estimate.
- `since_last_update_seconds` is elapsed time since the last case update, not proof of a human response.
- `time_to_resolution_seconds` is present only when `resolved_at` exists, and is not a contractual performance claim.
- `active` means `open`, `investigating` or `contained`. `contained` is an operator workflow label, not independently verified on-chain containment.
- `counts_by_status` and `counts_by_severity` count **returned rows only**; they are not an all-time case inventory.
- `sla_state=not_configured_no_deadline_or_assignment_claim`: no response/containment deadlines, breached SLAs, analyst assignments or L1/L2/L3 coverage are inferred.

The query accepts an exact status filter, uses a default limit of 100 and a hard cap of 250, and orders by latest `updated_at`. Missing persistence returns HTTP 503 rather than an invented empty/healthy queue. Responses are `Cache-Control: no-store`.

## Non-authority and preservation contract

GET cannot create, close, resolve, assign, or delete cases. It does not modify ARVIS decisions, evidence signatures, alerts, customer entitlements or chain observations. Operator severity is triage metadata, not proof of an exploit, criminal attribution or asset safety.

## Acceptance still open

1. Exact-head Go tests, `go vet ./...`, full regression tests and review.
2. Integration verification against migrations and the actual owner authentication boundary.
3. Explicit, versioned SLA policy and audit events before reporting SLA misses.
4. Auditable analyst assignment and acknowledgment events before claiming L1/L2/L3 operations.
5. Operator UI wiring with incomplete-coverage and not-configured labels.

## Regression commands

```sh
cd koschei/api
test -z "$(gofmt -l internal/handlers/owner_incident_triage*.go internal/handlers/owner_incidents.go)"
go test ./internal/handlers -run 'Test(BuildOwnerIncidentTriage|IncidentElapsedSeconds|OwnerIncidentTriage)' -count=1
go test ./... -count=1
go vet ./...
```
