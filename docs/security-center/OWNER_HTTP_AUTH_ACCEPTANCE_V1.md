# Koschei SOC — owner HTTP boundary acceptance v1

**Status:** Stacked draft candidate. Evidence is only established when the exact-head GitHub Actions job is successful; no production deployment is implied.

## Objective

The earlier SOC PostgreSQL acceptance (#1480) tested Go handler methods, persisted assignment events, manual escalations and owner review notes directly. Direct handler tests bypass `registerOwnerRoutes` and its HTTP middleware, so they cannot by themselves prove that unauthorized HTTP callers are rejected.

This additive slice exercises the actual registered owner routes through `securityHeaders(mux)`, with a disposable PostgreSQL 17 database already migrated by the separate `SOC PostgreSQL 17 Acceptance` CI job.

## Denial cases checked

The test confirms HTTP 404 and zero incident audit writes for anonymous:
- `GET /api/owner/incidents?triage=1` and `GET /api/owner/incidents/{id}`
- `GET /owner/soc-triage`
- `POST /api/owner/incidents` and `POST /api/owner/incidents/{id}` with a manual owner review, analyst assignment, or escalation request.

It also rejects invalid owner secrets, mismatched configured owner wallets, and a missing owner secret configuration. Denied responses may not include the private case title or owner console page body. The incident status remains unchanged after the denied requests.

## Authorized owner acceptance

With the existing configured owner authentication headers (not an analyst identity):
- Persist one owner review note, one L1 analyst **label**, and an L2 manual escalation request through the registered HTTP endpoint.
- Verify all three immutable timeline entries are present with the appropriate versioned payload keys.
- Read back the incident history and owner triage queue; require persisted `owner_manual_review_recorded` and `requested` state.
- Confirm the triage still says SLA is **not configured**.
- Confirm the private owner SOC HTML page remains accessible to the owner and uses `Cache-Control: no-store`.

The test runs under the explicit `KOSCHEI_SOC_TEST_DB_CONFIRMATION=isolated` flag and an **exact** loopback DSN for a GitHub Actions-only `koschei_soc_acceptance` database. It cannot accidentally operate against a different database. All created fixtures are uniquely identified and cleaned up only within that disposable test instance.

## Scope and limitations

The test proves the current **owner-authenticated HTTP boundary** with real persisted case and audit records in a GitHub CI runner. It does not create or prove independent analyst authentication, L1/L2/L3 RBAC, named analyst acknowledgement, SLA timers, incident-response staffing, notification delivery, production accessibility, or on-chain defensive execution. No owner-auth scheme changes, migrations, API route changes, ARVIS verdict modifications, Railway changes, service shutdowns, or main-branch edits are included.

## Review gates

Require PASS (not skipped) for `TestOwnerSOCIncidentHTTPAuthBoundaryPostgres17` and the three earlier SOC PostgreSQL cases, plus all existing security, API, format, release, and operator acceptance checks on the exact PR head before promotion.
