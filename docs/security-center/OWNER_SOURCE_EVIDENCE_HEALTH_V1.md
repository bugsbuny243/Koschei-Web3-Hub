# Owner Source Evidence Health V1 — independent completion, subcheck, and RPC visibility

**Dependencies:** #1404 → #1405 → #1406 → this stacked Draft. Parent PRs must be reviewed and merged in order before this child is rebased and merged. The source MHT exports, full user observations and any owner credentials are not committed.

## Reproduced evidence (17 October 2026)

Two **different token targets** were scanned; no same-token reproducibility conclusion is supported by this comparison.

| Source metric | PYUSD scan | ANSEM scan |
| --- | ---: | ---: |
| `investigation_coverage.completed / capability_total` | 7/14 | 11/14 |
| `evidence_pending`, `not_applicable` | 6, 1 | 2, 1 |
| `investigation_coverage.source_unavailable` | 0 | 0 |
| `full_scan_live_evidence.status` | complete | partial_timeout |
| `wallets_completed / wallets_requested` | 4/4 | 1/4 |
| `full_scan_live_evidence.rpc_failures` | 0 | 2 |
| Signed grade | D | D |

In ANSEM, **four completed arms** carry `evidence_status=partial_rpc_evidence_deadline`: `token_authority_scanner`, `holder_concentration`, `funding_cluster_detector` and `program_relation_scan`. This does not automatically mean the whole arm failed; completion is an execution classification, whereas source depth and subcheck assertions are separate evidence scopes.

ANSEM's `investigation_transparency.collection_gaps` includes a `context canceled` live actor RPC read and a Helius `-32015` unsupported transaction version **1** response. This display patch does not change `maxSupportedTransactionVersion=0` or attempt to parse unsupported transaction versions.

The display previously described as `Confidence NONE` is attributable in PYUSD's canonical report to `holder_cluster.confidence=none`. It is **not** `final_verdict.confidence`, which has no corresponding value in that report.

## Implementation

A new owner-only, read-only presentation addon `owner-source-evidence-health.js` attaches to the existing investigation coverage card without touching protected canonical projection or core Owner Radar Kit files. It independently shows:

1. Canonical ARVIS collector counts, including pending, N/A and unavailable.
2. Count and bounded module IDs of completed arms with partial/deadline evidence statuses.
3. Completed arms whose required subcheck success is **not confirmed** (unknown does not equal failed).
4. Separate live wallet/transaction evidence coverage, RPC failure count and scope.
5. Explicit unsupported-v1 collection limitation when present in bounded gap entries.
6. `holder_cluster.confidence` as **module-specific confidence**, not a final signed grade confidence.
7. A signed grade as metadata only, with no proof or completeness promotion.

Rendering uses DOM `textContent`, does not embed raw RPC replies, and does not make network requests. If canonical fields are missing, it does not infer success or fabricate counters.

This is an **observability and terminology fix**, not a collector fix, validator, RPC provider change, grade modification, token-2022 extension parser or final verdict verification. No DB schema, authorization, API route, protected JS, or ARVIS signing changes.

## Acceptance and limitations

Simulated-DOM Node regression covers synthetic 11/14 arms plus four partial statuses and 1/4 wallet with two RPC errors; synthetic 7/14 and 4/4; module-scoped confidence `medium` vs `none`; unsupported-v1 text without enabling v1; missing data; duplicate cleanup and untrusted module ID normalization.

Passing tests and CI demonstrates static handling of the supplied report structures. It is not production RPC validation or a same-target before/after replay. Further work should compare **the same mint at matched collector configuration/rule version**, persisting source request outcomes and distinct observation timestamps.
