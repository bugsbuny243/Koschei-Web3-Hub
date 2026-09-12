# Koschei Web3 Production Blocker — 2026-09-12

## Scope

This record captures the production persistence blocker observed while closing the 2026-09-11 audit findings F06 and F13.

## Observed production failure

Enabling application persistence through `APP_DATABASE_URL` caused the production service to fail closed during startup.

Railway runtime logs reported:

`configured application persistence is unavailable: db ping failed: pq: password authentication failed for user 'neondb_owner' (28P01)`

The application therefore did not become healthy while the persistence opt-in was enabled.

## Interpretation

- The repository persistence wiring is present.
- The production `DATABASE_URL` credential currently referenced for application persistence is not accepted by the Neon/PostgreSQL endpoint.
- This is a production credential/connectivity blocker, not evidence that application persistence code is absent.
- `APP_DATABASE_URL` was cleared after the failed deployment so the service can return to its previous stateless production mode.
- `ENTITLEMENT_DATABASE_URL` remains intentionally unset; it must not be silently pointed at the application database because entitlement storage is a separate fail-closed trust boundary.

## Audit impact

### F06 — public cases readiness

Status: `OPEN / PRODUCTION-CREDENTIAL-BLOCKED`

The public case registry defaults to the database backend and returns HTTP 503 with `configuration_status=database_unavailable` when application persistence is not connected.

### F13 — deployment acceptance

Status: `OPEN / PRODUCTION-CREDENTIAL-BLOCKED`

Transport and public pages can remain healthy in stateless mode, but the strict public-product acceptance gate remains red while `/api/public/cases` cannot obtain its persistence backend.

## Required closure evidence

F06/F13 must not be closed until all of the following are observed:

1. a valid production application database credential is configured;
2. application startup logs confirm persistence connection success;
3. `/health` remains healthy after persistence opt-in;
4. `/api/public/cases?limit=100` returns HTTP 200 with its valid registry contract;
5. `koschei/public-product` status is green;
6. no fallback silently weakens registry integrity or entitlement boundaries.

Do not record or commit plaintext database credentials in repository files, issues, logs, or acceptance artifacts.
