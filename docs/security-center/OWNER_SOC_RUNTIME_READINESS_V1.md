# Koschei Web3 SOC — runtime read-path readiness v1

**Status:** Draft candidate stacked after #1475, which depends on #1473 and #1472. No production activation, no automatic remediation, and no claim that all SOC components work end to end.

## Why this exists

Green code checks alone cannot prove that the deployed application can read real incidents, audit records, analyst labels and escalation requests. This candidate introduces an owner-protected **runtime read probe**, deliberately separate from operational acceptance.

## Private endpoint and UI

- `GET /api/owner/soc/readiness`: owner-only, GET-only, `Cache-Control: no-store`. It does **not** bypass the existing owner session/secret boundary.
- `GET /owner/soc-triage`: the existing private SOC page also queries readiness independently and renders its state as text, never as unsanitized HTML.
- A missing database or a failed query returns HTTP **503** with `ok: false` and `state: unavailable_not_accepted`. No leaked database errors or credentials.

## Measured read-only probes

With a five-second request-scoped timeout on the configured read store (or primary fallback):

1. PostgreSQL connectivity: actual `PingContext`.
2. `security_incident_cases` and `security_incident_actions` relation presence.
3. Actual read access to **both** tables, including whether at least one row exists in each.
4. A bounded **50-case latest-updated sample**, with no case identifiers, owner names, addresses, dossier content, or evidence exported in the readiness response.
5. An actual count of sampled cases with related persisted action rows, using the existing incident/action foreign-key link; a successful independent table read is insufficient.
6. Existing assignment/escalation projection queries on those case IDs, including failure if their persisted payloads cannot be decoded.

A passing read query does **not** verify all historical cases, all audit events, strong historical audit immutability, write transitions, source evidence integrity, chain head coverage, alert delivery, RBAC, analyst acknowledgements, SLA policy, signed verdict enforcement, or actual production deployment.

## Truth states

- `unavailable_not_accepted`: no DB or a failed probe. HTTP 503.
- `not_accepted_no_case_observations`: queries succeeded, but no case sample was observed. HTTP 200 only means the read path was queried successfully.
- `not_accepted_no_audit_event_observations`: at least one sampled case but no audit events in the audited table.
- `not_accepted_no_linked_sample_audit_events`: case rows and some audit rows exist, but none of the sampled cases has a persisted, related action record.
- `bounded_case_audit_link_observed_not_end_to_end_accepted`: table reads, a nonzero case sample, at least one **sampled case linked to a real audit row**, and latest projection queries completed. This does **not** mean that every sampled case has an audited action or that the system is production ready.

The response contains `schema_version`, `as_of`, `read_source` (read store vs primary fallback), bounded sample counts including `linked_audit_case_samples`, per-table read flags, `scope`, and `boundary`. No private operational content is returned. Because a read replica may lag, freshness and replication acceptance remain **unknown** without separate checks.

## Release gates

- Exact-head Go build, formatting and API/security/auth CI.
- Isolated-PostgreSQL read-only acceptance with `KOSCHEI_TEST_DATABASE_URL`, explicitly checking **not skipped**.
- Owner/anonymous access regression and HTTP 503 behavior.
- An independent integration test that creates a fixture in a **disposable test database**, exercises creation/assignment/escalation, confirms the readiness queries, and cleans up that fixture only. This candidate does not do any writes.
- Runtime observations against an authorized deployment after approval, followed by manual release review.

**No files or tables are deleted; no migrations, infra, secrets, background workers, autonomous on-chain actions, or existing endpoints are disabled.**
