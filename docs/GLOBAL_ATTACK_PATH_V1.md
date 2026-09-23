# Koschei Global Attack Path v1

Date: 2026-09-23

## Purpose

`koschei.global-attack-path.v1` turns the Global Entity Graph into a replayable ordered path.

The path engine is deliberately evidence-first. It does not infer that two addresses belong to the same attacker merely because they are correlated.

## Path stages

- funding
- deployment
- liquidity
- interaction
- drain
- swap
- bridge
- transfer
- holding

## Evidence rules

- Every segment comes from an existing graph edge.
- Segments must be contiguous: the previous target is the next source.
- Evidence edges produce an `EVIDENCE_BACKED` path.
- If any segment is a hypothesis, the whole path is `CANDIDATE`.
- Correlation and `suspected_same_actor` edges are never path-traversable.
- Cross-chain continuity therefore needs an explicit bridge edge rather than a same-actor guess.
- Every segment retains its source radar-event digests.
- The whole ordered path receives a deterministic SHA-256 digest.

## Discovery

`FindPaths` performs bounded deterministic traversal:

- maximum 12 hops;
- maximum 100 returned paths;
- cycle avoidance;
- evidence-only mode by default when `includeHypotheses=false`;
- shortest paths sort first.

This is the foundation for:

```text
funding
  -> deployment
  -> liquidity
  -> interaction
  -> drain
  -> swap
  -> bridge
  -> destination-chain transfer
```

The engine describes what the evidence graph supports. It does not label an entity as an attacker unless a separate evidence-backed decision layer establishes that claim.
