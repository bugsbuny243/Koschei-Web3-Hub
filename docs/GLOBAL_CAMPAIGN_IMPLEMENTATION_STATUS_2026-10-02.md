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

5. `feat/global-campaign-v1-lookup-merge`
   - PostgreSQL current-revision evidence-anchor index
   - DB-backed existing-campaign candidate lookup using canonical evidence references only
   - deterministic single-candidate merge across partial arrival windows
   - stable campaign identity and lifecycle-state preservation during material convergence
   - replay-stable no-op behavior when incoming evidence adds no material state
   - fail-closed ambiguous multi-campaign matches instead of silent identity collapse
   - fail-closed ruleset-version conflicts
   - sorted evidence-anchor advisory locking plus stable campaign-ref serialization
   - current revision reload under campaign lock before merge, preventing stale concurrent writers
   - PostgreSQL 17 race-detector acceptance for disjoint-anchor concurrent convergence
   - entity-only hints and verdict references excluded from automatic candidate matching

## Not yet implemented

- deliberate evidence-backed campaign-to-campaign alias/merge contract for resolving ambiguous identities
- ClickHouse-backed heavy campaign event history
- cross-chain temporal window correlation and campaign-window scheduling
- bridge adapter expansion
- Sentinel Fabric campaign evidence/opinion contracts
- campaign-to-incident linkage
- response policy/authorization
- independent containment effect proof
- operator command-center campaign view
- production 24/7 campaign runtime activation

## Persistence boundary

PostgreSQL stores only the stable campaign identity, immutable canonical revisions, the current projection, and a compact evidence-reference lookup index. Heavy/raw radar event history is not copied into campaign memory. A revision is accepted only when the canonical Global Campaign validator passes. Revision 1 must be the first persisted state; later writes must be exactly contiguous. An exact replay is idempotent, while a same-revision payload/hash mismatch, a stale write, a revision jump, invalid state transition, persisted metadata/payload/hash disagreement, ambiguous candidate set, or ruleset conflict fails closed.

Automatic candidate convergence requires shared canonical evidence references. Subjects, actors, assets, contracts, pools, bridges, and verdict references can be preserved as campaign context but cannot by themselves cause automatic campaign identity merging.

## Authority boundary

Global Campaign remains correlation context only. It cannot mint or modify ARVIS verdicts, grades, signing decisions, containment authorization, real-world identity claims, common-control claims, or wrongdoing claims.
