# Security Center — manual incident review candidates v1

**Status:** Additive owner-only read projection; operator action remains explicit.

## Endpoint

Use the already-authenticated existing route:

`GET /api/owner/incidents?suggestions=1&limit=50&target=<optional-exact-target>`

The default `GET /api/owner/incidents` response, all existing incident POST/action endpoints, owner middleware, and route inventory remain unchanged.

## What it does

- Reads existing `security_alert_events` high/critical records first created within the past 7 days.
- Suppresses alerts already linked to active incident cases (`open`, `investigating`, `contained`).
- Exposes only alert metadata and a **proposed** `ownerIncidentCreateRequest` body. An owner must separately choose to send `POST /api/owner/incidents`.
- The existing POST checks alert references against persisted records inside its own transaction and records a durable operator action.
- No incident row, alert, grade, signature, dossier, timeline, or response action is created/modified during GET.
- Never infers a network or external real-world identity from the alert's target, title, or payload. `network` remains blank in suggested drafts.
- Uses a bounded `created_at` indexed window, an exact optional target filter, and a maximum of 100 results.

## Truth and security boundaries

- A persisted alert and its severity are **not** independent proof of an exploit, compromise, malicious intent or chain safety.
- Deduplicated alerts created before the 7-day window do not appear, even if `last_seen_at` was recently updated. This is explicitly disclosed.
- Unresolved incident alert references are rejected by the existing POST.
- This endpoint inherits the existing owner-only and DB middleware; it is not public or customer-facing.
- The query is parameterized, no dynamic SQL, no raw alert payload, no background worker or new database migration.

## Validation

The new tests include pure response-boundary coverage, missing-DB behavior, and an optional PostgreSQL 17 integration test proving GET performs no incident writes and owner POST explicitly creates and links a case. In a full repository checkout:

```sh
cd koschei/api
gofmt -l internal/handlers/owner_incident_suggestions*.go internal/handlers/owner_incidents.go
go test ./internal/handlers -run 'TestOwnerIncident(ReviewCandidate|Suggestions)' -count=1 -v
go test ./... -count=1
go vet ./...
```

PostgreSQL integration requires `KOSCHEI_TEST_DATABASE_URL`. If absent, the integration test skips and **must not** be counted as PostgreSQL acceptance. Review CI evidence before promoting the PR.
