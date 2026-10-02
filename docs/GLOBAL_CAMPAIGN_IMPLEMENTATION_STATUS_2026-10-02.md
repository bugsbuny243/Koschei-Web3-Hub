# Global Campaign implementation status — 2026-10-02

## Implemented in stacked branches

1. `feat/global-campaign-v1-contract`
   - canonical `koschei.global-campaign.v1` contract
   - stable campaign reference and per-revision evidence hash
   - canonical reference normalization
   - hard non-authority boundary
   - fail-closed validation

2. `feat/global-campaign-v1-persistence`
   - deterministic lifecycle transition contract
   - evidence-bound reason/evidence references
   - immutable next-revision construction

3. `feat/global-campaign-v1-materializer`
   - reference-first deterministic materialization core
   - initial KCAM1 identity from canonical evidence anchors
   - order-independent canonicalization/hash behavior
   - stable existing campaign identity preservation

4. `feat/global-campaign-v1-store`
   - PostgreSQL append-only campaign revision ledger
   - atomically maintained current-state projection
   - per-campaign PostgreSQL advisory writer serialization
   - exact same-revision replay idempotency
   - fail-closed divergent, stale, skipped-revision and invalid lifecycle writes
   - strict canonical-payload and stored metadata/hash validation
   - PostgreSQL 17 acceptance coverage for revision persistence and corruption detection
   - compact canonical campaign memory only; no raw radar event-body duplication

## Not yet implemented

- DB-backed existing-campaign candidate lookup and deterministic merge/convergence across partial arrival windows
- ClickHouse-backed heavy campaign event history
- cross-chain temporal window correlation
- bridge adapter expansion
- Sentinel Fabric campaign evidence/opinion contracts
- campaign-to-incident linkage
- response policy/authorization
- independent containment effect proof
- operator command-center campaign view
- production 24/7 campaign runtime activation

## Persistence boundary

PostgreSQL stores only the stable campaign identity, immutable canonical revisions and the current projection. Heavy/raw radar event history is not copied into campaign memory. A revision is accepted only when the canonical Global Campaign validator passes. Revision 1 must be the first persisted state; later writes must be exactly contiguous. An exact replay is idempotent, while a same-revision payload/hash mismatch, a stale write, a revision jump, invalid state transition, or persisted metadata/payload/hash disagreement fails closed.

## Authority boundary

Global Campaign remains correlation context only. It cannot mint or modify ARVIS verdicts, grades, signing decisions, containment authorization, real-world identity claims, common-control claims, or wrongdoing claims.
