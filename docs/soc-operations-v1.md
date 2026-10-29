# SOC operations v1

This additive delivery extends the existing `security_incident_cases` workflow;
it does not replace Public SOC, owner incident actions, ARVIS or Fabric boundaries.
It introduces an owner-managed L1/L2/L3 queue and a polling case workspace at
`/owner/soc`, linked from the existing owner control center.

## Delivered behavior

- Existing incidents appear without backfill or changes to their source records.
  No SOC metadata means L1, unassigned, revision zero.
- Owners can assign/unassign a label, escalate to a higher level and explicitly
  acknowledge a response. A reason and expected revision are required.
- Updates and before/after audit events commit in one transaction. The same
  incident row lock used by existing close/reopen actions serializes edits;
  stale revisions return HTTP 409. Closed/resolved cases must first be reopened.
- The new event table rejects updates/deletes. Actor identity comes from the
  owner route boundary, never from request JSON. Assignees are labels: no
  multi-user analyst authentication, role delegation or invitations are claimed.
- The page shows evidence/alert/dossier references and the original incident
  timeline, and reuses existing note/status actions. SOC handoffs are separately
  paginated, newest first. Existing incident history remains limited to its
  existing first 500 actions, explicitly labelled in the UI.
- The queue refreshes every 15 seconds while visible. Case detail refresh is
  explicit to preserve drafts; optimistic locking protects concurrent SOC edits.
  This is not a push chat, staffed 24/7 operation or external war-room integration.

## First-response target

Policy `soc-first-response.v1` starts at incident creation, using severity from
the earliest persisted `created` action. A later severity change does not reset
the clock or hide a breach. These are internal default targets, not contractual SLAs.

| Initial severity | Target |
| --- | --- |
| critical | 15 minutes |
| high | 1 hour |
| medium | 4 hours |
| low | 24 hours |
| info | 48 hours |

The response is the earliest explicit SOC acknowledgement or persisted
`investigate`, `contain`, `resolve`, `close` action. A response at the deadline is
on time. An unanswered case at the deadline is overdue. Late responses remain
`breached` after closure. Reopening never resets this first-response measurement.
Assignment, escalation and notes do not themselves count as a response.

Missing initial severity, future/inconsistent timestamps and closed cases without
a response are `unknown`, never healthy. Old cases with valid history can already
be overdue on rollout: no artificial grace period or backdated acknowledgement is
created. Resolution-time targets and shift/business calendars are not implemented.

## API and persistence

All surfaces require the existing owner authentication and send `Cache-Control:
no-store`. Both `/owner/soc` and the physical `/owner-soc.html` are protected.

- `GET /api/owner/soc/queue`: `tier`, `status` (default `active`, also `all` or an
  existing incident status), `severity`, exact `assignee`, `unassigned=1`, `limit`
  (1–250, default 100), `offset` (0–100000). Stable priority/creation/ID order,
  `has_more`, and explicitly **returned-page** telemetry. This is not a global
  total. Concurrent case changes may move offset pages; refresh for current state.
- `GET /api/owner/soc/case?id=<uuid>`: case metadata and the latest 100 SOC events
  from a consistent read-only snapshot. `before_event=<id>` loads older handoffs.
- `POST /api/owner/soc/case?id=<uuid>`: JSON only, bounded to 16 KiB, unknown fields
  and trailing JSON rejected. Example:

```json
{"action":"assign","assignee":"Analyst A","summary":"Initial triage handoff","expected_version":0}
```

Use `action=escalate` with `tier=L2|L3`, or `action=acknowledge`. Blank `assignee`
with `assign` unassigns. `actor` cannot be supplied. All SOC reads use the primary
database to avoid replication lag after a write. No database or missing migration
returns 503; the page marks its data unavailable/stale instead of showing zero risk.

Forward migration `137_soc_operations.sql` creates two tables, one index and an
audit immutability trigger. It does not alter, delete or rewrite existing tables,
constraints, migrations, incidents, source evidence, verdicts or signatures.
Only new SOC metadata and SOC audit events are written by the new API.

## Validation and activation

```sh
go test ./internal/socops ./internal/http -run 'TestFirstResponse|TestSOC'
go test -race ./internal/handlers -run '^TestSOCOperationsPostgres$' -count=1
python3 scripts/check_migration_numbering.py
go run ./cmd/openapi-gen -check
```

The PostgreSQL test needs `KOSCHEI_TEST_DATABASE_URL` pointing to a disposable test
database with schema-creation rights. It creates/drops only its isolated random
test schema, applies migrations 123 and 137 there, and checks concurrency,
historical SLA calculation, filtering, input rejection and audit immutability.
API Required CI runs it explicitly on PostgreSQL 17.

Apply the new migration through the existing deployment process before using the
new workspace. No live database migration, service restart, configuration change,
deployment, contract pause, allowance revoke or transaction broadcast is part of
this source change. The legacy workflow continues to work if the SOC tables have
not yet been provisioned; the new workspace reports unavailable.

## Remaining SOC roadmap

The architectural target remains broader than this delivery. Next independent
increments should each include provider/runtime evidence and operator telemetry:

1. Named analyst identities, permissions, shift ownership and resolution targets.
2. Provider-backed archive/mempool and Web2 sensors, with freshness, gaps and
   replay health; do not label observe/synthetic sources live without proof.
3. Continuous invariant fuzzing and evidence-linked cross-chain fund tracing;
   bridges/mixers and shared counterparties are not proof of identity or guilt.
4. STIX/TAXII ingestion, provenance/expiry and evidence-backed threat profiles;
   sanctions matching needs authoritative data and disambiguation.
5. Separately authorized inline enforcement and pause/revoke adapters, with
   explicit wallet/contract authority, simulation, expiry and execution receipts.
6. Exportable incident evidence packs and external war-room connectors. No
   exchange freeze or third-party notification is sent by this delivery.
