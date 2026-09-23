# Koschei Global Entity Graph v1

Date: 2026-09-23

## Purpose

`koschei.global-entity-graph.v1` is the chain-independent relationship layer above Global Radar events.

It does not identify a human or claim common ownership. An entity is a normalized evidence subject scoped by:

`network + subject kind + canonical reference`

This prevents the same-looking address on different networks from silently collapsing into one identity.

## Hard boundaries

- Entity IDs are deterministic SHA-256 identifiers.
- Every node must come from an OBSERVED or VERIFIED Global Radar event.
- Every edge must cite one or more source event digests.
- Evidence edges and hypotheses are structurally separate.
- A hypothesis cannot carry OBSERVED or VERIFIED evidence state.
- Hypothesis confidence is bounded to 1–9999 basis points and must include a reason code.
- `suspected_same_actor` is always a hypothesis; it cannot be promoted to verified evidence by this graph.
- Cross-network edges are restricted to explicit cross-chain relations such as `bridged_to`, `correlated_with`, or `suspected_same_actor`.
- Literal same-address syntax across two chains does not mean same entity or same owner.

## Flow

```text
Global Radar Event
      ↓
network-scoped entity
      ↓
typed evidence edge OR explicit hypothesis edge
      ↓
canonical graph + graph digest
      ↓
cross-chain attack-path / fund-flow reasoning
```

## Initial relations

- funded
- transferred_to
- called
- created
- deployed
- bridged_to
- liquidity_to
- holds
- interacted_with
- correlated_with
- suspected_same_actor

The relation vocabulary is deliberately small in v1. New relations should be versioned or added with acceptance tests rather than accepted as arbitrary free-form labels.

## Next slice

The next layer should build an attack-path projection over this graph:

`funding -> deploy -> liquidity -> victim interaction -> exploit/drain -> swap -> bridge -> destination`

Only evidence-backed edges may form confirmed path segments. Hypothesis edges may appear as separate candidate branches with their confidence and evidence references preserved.
