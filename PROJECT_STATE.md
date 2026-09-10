# Koschei Web3 Project State

**Checkpoint date:** 2026-09-09  
**Branch:** `integration/unified-fabric-2026-09-09`  
**Status:** active integration work; not a production-completion claim.

This file supersedes the older uploaded-snapshot checkpoint for the current integration branch. Historical fixes remain valid where the code still contains them, but old statements that Koschei Sentinel is cancelled/frozen are no longer current project policy.

## CURRENT PROJECT BOUNDARIES

- Koschei Web3 / ARVIS remains the owner of Web3 security validation, evidence-backed deterministic customer decisions, product integration and case coordination.
- Koschei Lang remains a separate repository and owns language semantics, capability/authority, canonical runtime/IR and effect boundaries.
- Koschei Sentinel is reactivated and remains a separate repository. It owns threat/model/evaluation/research interpretation and must not mint Web3 decision authority or Lang runtime authority.
- Cross-project work uses Koschei Fabric versioned contracts/adapters. New integration starts observe-first and must preserve existing product behavior.
- Existing native v1 schemas are not silently widened. Cross-project fields use a separate `fabric.security-case-envelope.v1` binding contract.

## WEB3 RUNTIME BOUNDARY

- The Go Web3 runtime remains stateless with respect to blockchain/radar/evidence application persistence.
- `main.go` does not read the legacy application `DATABASE_URL` and does not start database-backed radar/job/alert workers.
- Durable jobs, historical readback, watchlists and other genuinely stateful application features remain fail-closed while application persistence is disabled.
- Existing synchronous evidence collection and deterministic analysis paths are preserved.
- Missing evidence remains UNKNOWN/withheld rather than being converted into a safe result.

## CORE-01 ENTITLEMENT SPLIT-PLANE — IN PROGRESS

The prior stateless architecture had a concrete contradiction: customer routes such as `/api/v1/radar/check` were allowed to execute without the application database, but the Professional entitlement/output gate still queried `Handler.DB`. With application `DB=nil`, authenticated paid requests therefore failed at `plan_access_unavailable` before analysis.

The integration branch now separates commercial authorization from application persistence:

- `Handler.EntitlementDB` is a dedicated commercial authorization ledger handle.
- `Handler.DB` remains the application persistence handle and may stay nil.
- `ENTITLEMENT_DATABASE_URL` is the only new runtime configuration used by the stateless process for paid-access enforcement; there is no implicit fallback to `DATABASE_URL` in `main.go`.
- `ConnectEntitlementStore` opens an existing ledger without running application migrations and verifies the required `entitlements`, `app_user_profiles`, and `credit_events` shape.
- plan evaluation, atomic output reservation and refund use the entitlement ledger.
- existing stateful deployments retain backward compatibility because entitlement access falls back to `Handler.DB` when no dedicated handle is supplied.
- if no entitlement ledger is configured, paid customer operations remain fail-closed; no plan or quota is fabricated.
- if an explicitly configured entitlement ledger cannot connect or does not expose the required schema, startup fails rather than silently weakening authorization.

This is an implementation slice, not CORE-01 completion evidence. A real configured-ledger acceptance run and billing/entitlement lifecycle verification are still required.

## KOSCHEI FABRIC SECURITY WORK

The integration branch contains a machine-readable security work index derived from `Koschei-Web3-Web6-Guvenlik-Calisma-Paketi-2026-09-08.md`:

- 12 security domains / 48 work packages;
- P0 blockers: `CORE-01`, `CORE-02`, `CORE-04`, `SIGN-01`, `MODEL-04`, `LANG-01`, `LANG-02`, `SUPPLY-02`;
- implementation packages A through H;
- first common acceptance cases T01 through T14;
- lifecycle state and evidence maturity are tracked separately.

Current package state on this branch:

- A — status record correction: **in progress**;
- B — working Web3 foundation: **blocked pending acceptance**, with CORE-01 implementation now in progress;
- C — shared data boundary: **in progress**;
- D–H: planned/gated by their prerequisites.

## SHARED CASE / EVIDENCE BOUNDARY

`fabric.security-case-envelope.v1` is a strict cross-project binding envelope. It does not replace project-native schemas or create a shared decision authority.

- Web3 projection owns case/request/effect and its native ARVIS binding.
- Lang projection only exports an already-evaluated Lang-native authority/runtime result; it cannot derive authority from Fabric JSON, Sentinel output or tool metadata.
- Sentinel projection is structurally `evidence-interpretation-only`; it does not produce authority.
- digest, freshness, mapping and independent effect-observation states are explicit.
- a `VERIFIED` independent effect observation requires a receipt digest.

The adapters and schema are implementation artifacts only. T01–T14 have not yet been claimed as executed acceptance evidence.

## VERIFIED SO FAR

Before the latest CORE-01 split-plane commits, the Fabric branch had already passed the repository's OpenAPI, Auth Freeze, Supply Chain, CodeQL, API Required CI, Operator Exit Corpus, Release Gates and Security CI checks. A prior Public Product Smoke failure was isolated to the deployed public case registry returning HTTP 503 while the other probed product pages/assets returned 200.

The latest CORE-01 commits have started a new CI cycle. Do not treat the earlier green run as validation of these newer commits; exact-head CI must pass again.

Koschei Sentinel PR #94 currently passes its general CI plus PQ evidence-review and PQ reviewed-catalog admission checks with the new Fabric case adapter included.

## STILL OPEN / BLOCKED

- CORE-01: real configured entitlement-ledger acceptance and full billing entitlement lifecycle validation.
- CORE-02: exact-head readiness/CI and deployed-state reporting must stay truthful.
- CORE-04: shared schema/adapters exist, but common acceptance cases have not yet been executed end-to-end.
- SIGN-01: intent/signed-bytes/executed-effect one-use acceptance remains open.
- LANG-01 / LANG-02 / SUPPLY-02: canonical execution, physical broker/OS isolation and independent release trust-root acceptance remain open in Koschei Lang.
- MODEL-04: Koschei Sentinel 397B total / 35B active remains an architecture target; real candidate training/checkpoint/HOLDOUT evidence is still required.
- Durable Web3 jobs/history/watchlists remain unavailable without their deliberately separate application persistence/worker path.

## WORK-IN-PROGRESS POLICY

1. Do not disable or remove existing working product behavior to add Fabric capabilities.
2. Keep `main` untouched until the integration candidate passes its required gates and review.
3. Do not fabricate database state, jobs, evidence, entitlement, model readiness or security acceptance.
4. Preserve ARVIS evidence semantics and UNKNOWN on missing evidence.
5. Keep Lang, Sentinel and Web3 ownership boundaries explicit; integrations use versioned contracts/adapters only.
6. A file, schema or test definition is not completion. Completion requires the exact candidate commit, executed acceptance evidence and remaining limits in the same record.
