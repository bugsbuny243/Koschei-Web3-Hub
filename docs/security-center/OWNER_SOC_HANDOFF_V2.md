# SOC operations handoff v2

`GET /api/owner/incidents/{uuid}?handoff=2` exports a bounded owner-only snapshot of the incident, its original action history, and SOC operations. The case workspace at `/owner/soc` exposes this export. It does not send the package to another party.

The existing `handoff=1` format and ordinary incident reads remain available. V1 omits operations and retains its original schema and checksum contract. V2 uses `koschei.soc-incident-handoff.v2` in both the envelope and hashed content. No migrations or changes to source incidents are required.

## Snapshot and content

V2 reads the primary database using one `REPEATABLE READ READ ONLY` transaction, with a five-second request budget. This avoids exporting a stale read replica immediately after a queue action. The existing `incident` and `actions` fields are retained, with an additional `content.operations` object:

| Field | Meaning |
| --- | --- |
| `version` | `soc-operations.v1` contract |
| `read_source` | `primary_db` |
| `as_of` | UTC time used for the first-response SLA evaluation |
| `state` | Current tier, assignee label, acknowledgement timestamp and revision |
| `initial_severity` | Severity from the original creation event; empty if unavailable |
| `sla` | Existing first-response policy result, deadline and response time |
| `events` | Full bounded SOC history, ordered by revision; includes actor, summary, before/after state and timestamp |

The SHA-256 covers the complete serialized `content`, including operations and its evaluation time. Consequently, exports taken at different times may have different digests even without a new action. This checksum is not a signature, proof of actor identity or independent evidence verification. Assignment labels in legacy action payloads and operations are retained in their own records; this export does not reconcile or overwrite them.

## Failure behavior

- Missing operations tables or unreadable data return 503, without a digest or fallback to v1.
- Revision gaps, invalid state transitions, or a final event state inconsistent with current metadata return 503. Cases with no operations start at L1/revision zero with an empty events array.
- More than 500 original actions or 500 SOC events return 409; histories are never silently truncated. Each history has a separate 2 MiB payload/text budget; SOC before/after JSON combined is limited to 128 KiB per event.
- Unknown original severity or invalid response timestamps retain the existing `unknown` SLA result.
- Responses use `no-store` and `nosniff`; existing owner authentication applies. Export does not grant L1/L2/L3 access roles or authority to pause contracts, revoke allowances or freeze exchange funds.

## Verification

The required PostgreSQL 17 `TestSOCOperationsPostgres` acceptance test covers missing schema, v1 compatibility, assignment/escalation/acknowledgement export, digest recomputation, metadata/history mismatch, oversized events and the 501-event limit. Unit tests cover broken revision chains and transitions. HTTP route tests cover unauthenticated denial and unavailable-store behavior for v2.
