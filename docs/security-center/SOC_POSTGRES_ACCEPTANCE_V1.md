# Koschei SOC — PostgreSQL 17 integration acceptance (candidate)

**Status:** GitHub Actions test gate proposed in a stacked draft PR. This document is not evidence of successful execution. Verify the job's exact commit SHA and uploaded log before claiming acceptance.

## Why this gate is required

The parent PRs (#1472, #1473, #1475 and #1479) have successful regular CI runs. However, three SOC integration tests were written to **skip** when `KOSCHEI_TEST_DATABASE_URL` was absent; a green `go test ./...` does not establish that those particular tests exercised a database.

The separate `SOC PostgreSQL 17 Acceptance` workflow closes this specific gap by failing if one of the three named tests is skipped. The workflow does not deploy or modify production.

## Database isolation

- GitHub Actions launches a disposable `postgres:17` service with a single `koschei_soc_acceptance` test database, bound only to runner loopback port 5432.
- The job applies the repository's **existing SQL migrations** with stop-on-error, then confirms the incident tables are present.
- The tests require `KOSCHEI_SOC_TEST_DB_CONFIRMATION=isolated` and a DSN targeting `127.0.0.1`, `localhost` or `::1`, port 5432, database `koschei_soc_acceptance`, with SSL disabled for runner-local transport. Any different non-empty DSN fails **before fixture writes**, rather than falling back to a live database.
- Fixtures use a unique incident ID and cleanup of only their own test-created case. No existing case or production data is used.

## Evidence-bearing test sequence

The workflow executes three named PostgreSQL integration tests (without allowing silent skips):

1. **Assignment:** create a case, assign L1, reassign L3, unassign, read back the latest state and all three append-only audit events.
2. **Escalation:** refuse a request without assignment, assign L1, request L2 manual escalation, reassign L2, confirm the read projection, clear the request, reject repeated clear and verify the complete action history.
3. **Owner manual review:** confirm no fabricated review record, create two manual owner review notes, verify the first/last timestamps and count, inspect the timeline, and reject review after resolution.

A separate Go test confirms the private SOC HTML route rejects anonymous users and retains its CSP protections.

The workflow uses exact required `--- PASS` log markers for all three PostgreSQL tests and rejects `--- SKIP:` in the captured output. The log is uploaded as a short-retention artifact for acceptance review.

## Boundary and remaining limits

Even successful acceptance would prove **the test fixtures and code path on disposable PostgreSQL**, not production runtime behavior, real analyst authentication, tested role-based permissions, deployed incident workflows, accepted response-time SLAs, external notification delivery, or on-chain blocking.

The user-facing owner SOC queue remains read-only, owner-authenticated, and expressly differentiates manual labels and review notes from verified analyst actions.

No migration files, production variables, Railway resources, source evidence, ARVIS deterministic verdicts, token controls, or main-branch code are changed by this workflow.
