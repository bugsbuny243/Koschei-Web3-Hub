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

## Not yet implemented

- PostgreSQL campaign identity/current-state/revision persistence
- existing-campaign lookup and candidate merge/convergence across partial arrival windows
- ClickHouse-backed heavy campaign event history
- cross-chain temporal window correlation
- bridge adapter expansion
- Sentinel Fabric campaign evidence/opinion contracts
- campaign-to-incident linkage
- response policy/authorization
- independent containment effect proof
- operator command-center campaign view
- production 24/7 campaign runtime activation

## Authority boundary

Global Campaign remains correlation context only. It cannot mint or modify ARVIS verdicts, grades, signing decisions, containment authorization, real-world identity claims, common-control claims, or wrongdoing claims.
