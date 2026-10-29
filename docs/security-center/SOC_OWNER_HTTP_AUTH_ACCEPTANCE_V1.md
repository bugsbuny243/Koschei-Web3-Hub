# SOC — owner HTTP authorization acceptance v1

**Status:** Draft acceptance candidate, stacked after the PostgreSQL incident lifecycle gate in PR #1480. This is test-only; no production authorization is changed.

## The validation gap

The owner SOC assignment, escalation and review acceptance scenarios exercised the Go incident handler directly. Direct handler tests **bypass** the registered owner middleware. They cannot establish whether HTTP clients lacking owner credentials can read or mutate incident data.

The existing `/api/owner/incidents`, `/api/owner/incidents/{id}`, and `/owner/soc-triage` routes are kept intact. The new `TestOwnerIncidentHTTPAuthPostgres17` runs through the existing `registerOwnerRoutes` router, `ownerOnly` authorization guard, `requiresDB` guard, `securityHeaders`, the real handlers and disposable PostgreSQL 17.

## Evidence assertions

The test creates **one uniquely identified disposable incident** after verifying the isolated test database URL and explicit confirmation. It then:

1. Confirms anonymous and wrong-owner-secret GET requests receive 404 on the private incident, triage API, and owner SOC HTML.
2. Confirms anonymous and wrong-owner-secret POST requests cannot assign an analyst, record an owner review, or request escalation.
3. Confirms an unverified Bearer header alone does not grant the owner mutation permission.
4. Reads PostgreSQL directly to prove denied requests inserted **zero** incident actions and left the case `open`.
5. Sends owner-authenticated HTTP POST requests to assign a manual L1 label, record an owner review, and request a higher L2 manual escalation.
6. Confirms the owner-authenticated HTTP timeline contains exactly three `note` actions with `actor='owner'`.
7. Confirms the owner-authenticated triage HTTP response derives matching manual L1 assignment, pending L2 escalation and one owner-review record from persisted history.
8. Verifies owner SOC HTML is not a write surface and did not append any actions.

The GitHub Actions `SOC PostgreSQL 17 Acceptance` workflow explicitly requires `--- PASS: TestOwnerIncidentHTTPAuthPostgres17` in its log, refuses `--- SKIP:`, and retains the short-lived log alongside the existing PostgreSQL acceptance evidence.

## Isolation

The test requires a disposable loopback PostgreSQL 17 service, the exact database `koschei_soc_acceptance`, port 5432, `sslmode=disable` as the only URL query parameter, and `KOSCHEI_SOC_TEST_DB_CONFIRMATION=isolated`. It refuses any other target **before** opening the connection or writing fixtures. The test-only owner credential is a fixed non-production literal, not a deployment secret. Cleanup is restricted to the test's own incident ID and unique title.

## What this does not prove

Passing the acceptance test demonstrates the registered HTTP middleware and relevant workflow in a controlled local test environment. It does not prove production security, actual user sessions, customer entitlements, analyst L1/L2/L3 RBAC, CSRF defense across production login flows, a staffed SOC, notifications, SLAs, external delivery, chain enforcement, or operating status.

No authentication implementations, deployment configuration, ARVIS evidence/verdict semantics, production tables, repository files, Railway resources or `main` branch are removed or modified as part of this test-only change.
