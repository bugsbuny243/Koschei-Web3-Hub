# Security Center integration readiness bridge v1

Status: additive, read-only, review required. This is **not** a certification, a security verdict, or a claim that all collectors are operational.

## Why this exists

The existing `koschei.security-center-capability.v1` registry identifies repository code paths; the existing `koschei.runtime-health.v1` registry reports operational component observations. Neither alone can establish whether a full security capability is available, effective, or accepted.

This change **joins without replacing** either record through:

`GET /fabric/security-center/integration-readiness`

The endpoint is exposed only through the existing Fabric mount. All existing routes, collectors, alerts, incident actions, customer APIs, verdict policies, database tables, and static assets remain unchanged.

## Evidence and trust policy

- The six explicitly mapped capability IDs carry real component IDs already registered in code; all others report `not_instrumented`, **not** `offline`.
- Missing runtime registry reports `runtime_registry_unavailable`; a registry that exists but has no matching component reports `not_reported`.
- With multi-component mapping, one absent dependency yields `partially_reported`. A degraded, unavailable or coverage-attention component yields `attention_required`, even if another component was live.
- Disabled/stopped reports `inactive`. Configured without a successful cycle reports `awaiting_observation`.
- `observed_operational` requires a reported live component with at least one successful cycle; it never means full chain coverage, complete scan, successful prevention, or accepted control proof.
- `acceptance_verified` is always false in this projection. It cannot promote controls from the separate 34-area/170-control acceptance catalog.
- Raw provider error strings, URLs, credentials and DSNs are not returned. Only fields from `runtimehealth.PublicSnapshot()` are projected.

## Explicit runtime links

| Registered capability | Existing runtime probe ID |
| --- | --- |
| continuous-multi-network-block-ingest | worker.global-radar-head-ingest |
| global-radar-coverage-alert-lifecycle | worker.global-radar-coverage-alert-lifecycle |
| global-campaign-runtime | worker.global-campaign-materializer; worker.global-radar-campaign-handoff |
| cross-process-job-wake | worker.cross-process-job-wake |
| network-and-validator-telemetry | worker.global-radar-background-telemetry |
| durable-evidence-memory | storage.application-postgres |

The last link observes **only the application Postgres dependency**. It does not establish completeness or health of all durable evidence stores.

## Review and promotion gates

The change is packaged in a draft PR for review and CI. Run in `koschei/api` with repository Go toolchain (1.26.9):

```sh
test -z "$(gofmt -l .)"
go test ./internal/securitycenter ./internal/runtimehealth ./internal/http -count=1
go test ./... -count=1
go vet ./...
go run ./cmd/openapi-gen -check
```

No database migration, new feature flag, private key, customer route, or background worker is introduced. Subsequent expansion should attach richer evidence or G01-G08 acceptance proofs via separate reviewed, fail-closed changes, not inferred from runtime state.
