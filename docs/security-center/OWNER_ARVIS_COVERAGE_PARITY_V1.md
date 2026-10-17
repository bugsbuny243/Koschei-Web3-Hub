# Owner ARVIS runtime vs investor checklist coverage — separate evidence views

**Stack dependency:** PR #1404 (backend runtime truth) → this PR. Draft until parent has been reviewed and merged/rebased.

## Contradiction reproduced

The source owner export from 17 October 2026 reports `investigation_coverage` with 14 architectural arms, **7 completed**, **6 evidence pending**, **1 not applicable**, zero source-unavailable and two critical gaps. A second widget reports **7 complete · 7 partial · 0 N/A**. These are not compatible descriptions of the same 14 ARVIS arm executions.

The second counter is derived from a **separate investor-facing checklist** (`KoscheiVerdictCard.mapVerdictCard(...).coverage`), not the authoritative `services.BuildArvisInvestigationCoverage` collector status. It is not legitimate to present the investor checklist as 14-arm operational coverage.

## Additive owner display correction

`public/js/owner-verdict-card-coverage.js` remains an independent addon around `OwnerRadarKit`. It now shows two explicitly separated lines in the verdict card header:

- **ARVIS collector kapsamı**: requires internally consistent numbers from the canonical `investigation_coverage` response. Shows completed/total, pending, not applicable, unavailable, insufficient, critical gaps and the reported terminal category. If missing/inconsistent, fails closed with an unavailable/inconsistent message.
- **Kontrol listesi (14 ARVIS kolundan ayrı)**: the existing investor-friendly checklist state, as already computed by `KoscheiVerdictCard`. It is not cast to a collector status.

No rewrite of the protected canonical projection script, no override of signed verdict grade or rules. Existing copyable evidence refs, DOM hierarchy, ARVIS engine and independent owner rendering are preserved. Runtime text is emitted via `textContent`, while the existing checklist string keeps HTML escaping.

## Acceptance

`scripts/verify-owner-arvis-coverage-parity.js` exercises the actual browser addon in a sandboxed simulated DOM: the production-shaped 7+6+1 case, 13 completed + 1 not applicable, missing/inconsistent counts, source-unavailable and escaped untrusted checklist input. A new GitHub Action runs it on changes.

**Scope boundary:** This is not new RPC evidence or more working arms. It fixes **what we claim about runtime completion**. The separate owner HTML export is not published. No DB changes, auth changes, worker changes, user-data exports or production merge.

Authority addresses appearing under discovery-only creator data need separate original RPC verification before being labeled canonical authorities; that concern is not merged into this coverage fix.
