# Koschei SOC — HTTP owner authorization + persistence acceptance

**Status:** Stacked draft test candidate, not deployed. Parent #1480 proves the handler-side incident lifecycle against disposable PostgreSQL 17 but initially did not verify unauthorized requests through the real registered HTTP middleware.

## Goal

Verify that `ownerOnly` on the existing incident routes prevents anonymous, invalid owner-secret, forged analyst/owner headers, and invalid bearer credentials from **reading private incident data or writing action history**. Then verify that an authenticated owner can submit the same commands and that their committed data appears in the authenticated triage projection.

## Acceptance target

`TestSOCIncidentOwnerAuthorizationPostgres17` is executed by the `SOC PostgreSQL 17 Acceptance` GitHub workflow, after applying the existing migrations to the disposable database. The test refuses an unconfirmed, non-loopback or differently named database before creating any fixtures. The test uses an ephemeral owner credential held **only in the test process** (not a production secret or added deployment variable).

- Register the **actual** `registerOwnerRoutes` and `securityHeaders` middleware using `httptest` and the disposable PostgreSQL pool.
- Create a unique test incident directly in the disposable store, never in production.
- Attempt owner triage GET, item GET, three incident POST actions, and the protected SOC HTML page without an owner credential and with invalid/spoofed credentials. All must return 404, with no case content exposed.
- Check that these unauthorized attempts did **not** append any case audit action.
- With a valid test owner secret, assign L1, request L2 escalation, and record an owner manual review through the registered **HTTP routes**; assert three persisted actions and confirm all three audit projections in the owner-only triage JSON.
- Require `Cache-Control: no-store` on the authenticated triage response.
- The workflow explicitly requires `--- PASS: TestSOCIncidentOwnerAuthorizationPostgres17`; any `SKIP` causes a failed job.
- Fixture cleanup deletes **only** the isolated, uniquely identified test case.

## What successful CI proves

The registered HTTP route, legacy owner-secret authentication middleware and PostgreSQL incident storage deny the tested forged calls while permitting the tested authenticated owner workflow, in the disposable GitHub Actions environment. The page-specific CSP test runs alongside it.

## What it does not prove

It does not validate production deployment, browser login/session cookies, public-client isolation across all routes, real analyst RBAC, an authenticated L1/L2/L3 account, incident-service horizontal concurrency, third-party notification receipt, SLA compliance, automatic response, chain coverage or on-chain enforcement. All remain separate acceptance gates.

No migrations, existing account permissions, signed ARVIS verdicts, Railway settings, live services, or `main` branch are modified.
