# Global Radar consolidation — 2026-09-24

This branch consolidates the remaining stacked Global Radar work after Phase 1 and the common event envelope were merged to `main`.

Included here:

- Global Entity Graph v1
- evidence-first Global Attack Path v1
- ARVIS path-evidence promotion boundary
- truthful operator surface
- live EVM and Bitcoin probe-to-radar-event projection
- Sui and Aptos network-identity event projection
- EVM execution-node and Ethereum Beacon telemetry adapters

## Integration boundary

The branch is rebased directly on current `main`. It preserves the already-merged universal Move dispatch and Global Radar observation layers instead of replacing them with older stacked copies.

Evidence and hypotheses remain separate. Candidate/hypothesis paths cannot mint ARVIS security evidence. Live network capability, provider configuration, observed evidence, verified evidence and deterministic ARVIS decisions remain distinct states.

## Superseded stacked review slices

The functionality previously split across PRs #1216 through #1221 is consolidated into the final Phase 3–9 PR so the repository does not retain a chain of half-finished stacked reviews.
