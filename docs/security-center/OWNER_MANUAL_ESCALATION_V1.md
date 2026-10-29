# SOC owner-reviewed escalation request v1

**Delivery status:** Draft, stacked after PR #1473. A code candidate, not a deployment, active SLA, or staffed SOC capability.

## Authority / boundaries

- Existing owner authentication protects the `/api/owner/incidents*` handlers. No new analyst login or permission grant is added.
- Existing PostgreSQL `security_incident_actions` accepts only its existing action-type allowlist from migration 123; the new requests are stored as existing `note` events containing a versioned `soc_escalation_v1` object.
- ARVIS verdicts, signed evidence, alert sources, on-chain state, outside notifications, contract pauses, customer actions, and historical incident actions are **not** changed.
- An escalation request is **not** a dispatch, analyst acknowledgement, response success, or proof of SLA adherence.

## Owner commands

On the pre-existing owner-authenticated `POST /api/owner/incidents/{id}`:

```json
{
  "action": "request_escalation",
  "tier": "L2",
  "summary": "The L1 incident review needs a higher-tier manual analysis."
}
```

Requires an active case (`open`, `investigating`, `contained`), a **persisted current analyst assignment**, and a strictly higher target tier: L1→L2/L3 or L2→L3. The request rationale must contain at least 8 non-whitespace characters (maximum 4000). The request does not switch assignments or grant access. All checks occur inside the existing row-locked incident transaction.

To explicitly clear a manual request without claiming work was performed:

```json
{
  "action": "clear_escalation",
  "summary": "Owner review: the manual request is no longer needed."
}
```

Requires an active case with the latest escalation ledger state `requested`. The clear operation writes a new audit event and never rewrites the previous request.

Both calls persist a `note` event with one of the following `payload.soc_escalation_v1` structures:

```json
{"state":"requested","target_tier":"L2"}
```

```json
{"state":"cleared"}
```

## Read-side projection

`GET /api/owner/incidents?triage=1` gains an optional `items[].escalation` field based on the latest **persisted** `soc_escalation_v1` audit event. Each event has a timestamp. Possible read-side states:

- `requested`: active request, with no evidence of delivery/acknowledgment.
- `cleared`: owner cleared the request.
- `historical_request_case_inactive`: case resolved or closed without a later clear event.
- `superseded_by_later_assignment_not_acknowledged`: a **later persisted assignment at or above the requested tier** supersedes the pending queue hint. This is not analyst acknowledgement or proof that the request was delivered. A later lower-tier assignment or unassignment does **not** clear the original request.

The projection returns HTTP 503 if the escalation audit cannot be queried or validated; missing events are omitted. The incident queue is still a **bounded window** and does not represent all historical cases. The `sla_state` remains unconfigured: **no timers, breach labels, human-response targets, or automatic escalation** are implemented.

## Private owner SOC view

`GET /owner/soc-triage` is a new, isolated **owner-authenticated**, read-only HTML page. It requests the existing `GET /api/owner/incidents?triage=1&limit=100` endpoint using same-origin credentials. The page displays returned case severity/status, persisted age, historical analyst labels, latest manual escalation state, and count of evidence references. It deliberately does **not** claim evidence verification, severity correctness, staffing, an active SLA, acknowledged escalation, or technical containment.

It has no POST actions, no client-side verdict engine, no automatic paging of historical cases, and no user-provided HTML injection. Non-owner page access is denied by the existing `ownerOnly` gate; HTML is `no-store` and uses the repository's nonce-based CSP processing.

## Validation

- Unit tests cover tier ordering, structured audit schema, invalid inputs, non-acknowledgment semantics, inactive cases, and unchanged evidence references.
- Required before promotion: exact-head Go tests and formatting; PostgreSQL request→inspect→clear integration test; role/owner authentication regression, concurrency review, owner UI wiring and deployment acceptance.

This is a safe owner-controlled operational workflow foundation, not an enterprise SOC certification.
