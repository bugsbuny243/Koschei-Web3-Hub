# Repository audit reconciliation — 2026-10-30

This is a **delta record**, not a declaration that all findings in the historical 2026-10-06 audit are fixed.

## Baseline

The uploaded `repo-denetim-ham-listeler.md` measured `main@9a19ed7` and reported 215 unreachable functions, 353 test-reachable-only functions, 32 unreferenced public assets, and six frontend/backend route mismatches. Those counts **must not** be treated as counts for current `main` without rerunning reachability and frontend reference checks against the exact current commit. This repository now locates the Go runtime and static frontend under `koschei/api/`.

## Verified current-code examples

- `ApplyCrossTokenCreatorHolderTransferRuleV120` is called by `internal/handlers/unified_investigation_report.go`.
- `parsePumpPortalEvent` is called by `internal/services/pumpportal_observable_client.go`. This proves a production call site, **not** provider connectivity or worker activity.
- `ProbeSolanaValidatorTelemetry` and `ProjectSolanaValidatorTelemetryToGlobalRadar` still require explicit production orchestration, bounded provider budgets and operator telemetry. No live rollout is claimed here.
- `EvidenceBackedFinalSecurityRadarVerdict` has tests, but being present does not prove that every customer response uses it. Existing `EvidenceBackedSecurityRadarBundleContext` is wired into the holder intelligence path and evidence gates must remain fail-closed.

## HTTP/static containment in this change

Existing `sensitiveStaticProbePath` already rejects test and fixture directories, test-module suffixes, repository config files and `validation-key.txt`. This change additionally returns 404 for ten legacy/test-only JavaScript entrypoints that no canonical HTML page loads:

- `koschei-modern.js`
- `customer-investigation-ux-v2.js`
- `customer-transaction-preflight-v1.js`
- `evm-authority-desk.js`
- `lp-control-evidence-card.js`
- `premium-evidence-matrix.js`
- `public-scan-request-guard.js`
- `unified-live-evidence-card.js`
- `verdict-card-evidence-refs.js`
- `verdict-card-market-context.js`

Sources remain in Git for tests and future deliberate integrations; they are **not** advertised as working product modules. Canonical `/js/koschei-auth.js`, `/js/customer-universal-address-scan-v1.js`, and `/js/public-solana-scan.js` remain accessible. The existing SDK path stays accessible because SDK compatibility is a separate decision. Unlinked HTML pages are not automatically deleted or blocked: route compatibility needs its own review.

## API route reconciliation

- `/api/public/tool-prices` and `/api/wallet/score` are registered in `internal/http/server.go`. Their presence is not an entitlement or end-to-end execution acceptance.
- `/api/mev/analyze`, `/api/liquidity/analyze`, and `/api/dao/proposal-risk` are intentionally quarantined; their synthetic analysis handlers must **not** be made live to satisfy an old checklist.
- `/api/owner/kosch-access` is not a canonical owner endpoint. The active owner auth/API surface must not silently reintroduce it.
- `frontend_route_quarantine_test.go` scans public HTML and JavaScript to prevent those four retired API paths from reappearing.

## Acceptance and remaining work

The code changes require exact-PR-head Go tests, relevant CI gates, and a new reachability audit before merging. The outstanding A/B functions require individual categorization (prod-intended, compatibility, test helper, explicitly retired), verified runtime call paths, source/UNKNOWN semantics, authorization and budget controls, and tests that exercise **registered routes** rather than invoking the helper in isolation. None of these 568 functions are declared universally production-wired by this static containment patch.
