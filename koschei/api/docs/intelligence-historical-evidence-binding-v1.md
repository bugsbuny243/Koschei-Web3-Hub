# Historical intelligence evidence binding v1

## Purpose

This layer defines how already-validated ClickHouse ARVIS memory may enter a fresh `koschei-intelligence-contract-v1` investigation without acquiring decision authority.

The binding is intentionally evidence-only. It exists between the ClickHouse memory projection and any future live investigation wiring.

## Contract

`BindClickHouseHistoricalEvidence` accepts:

- a live `IntelligenceInvestigation` produced from fresh ARVIS collection;
- a historical `IntelligenceInvestigation` produced by the bounded ClickHouse memory projection.

The function validates the complete binding before mutating the live investigation. If any check fails, the live investigation is left unchanged.

## Exact subject boundary

Every historical subject must exactly equal an already-present live subject, including its stable ID, raw chain-native identifier, canonical reference, chain family, chain and network.

Historical evidence must reference one of those exact subjects and must carry the same chain family, chain and network. No fuzzy address matching, case folding, entity inference or cross-network widening occurs here.

## Authority boundary

Historical input is accepted only when:

- it uses `koschei-intelligence-contract-v1`;
- it contains no entities, relationships, behavior findings, hypotheses or attack paths;
- its decision remains the default unverified `investigate` state;
- each imported evidence object remains `observed`;
- its source and provenance match the trusted ClickHouse ARVIS memory projection contracts.

Historical verdict evidence must additionally retain:

- `memory_authority=historical_context_only`;
- `signature_verification=not_performed_by_memory_projection`.

A stored signed verdict therefore remains historical metadata. It cannot become a fresh verified decision through this binding.

## Duplicate and conflict handling

Evidence IDs are stable identities.

- An exact duplicate is idempotently skipped and counted in the binding receipt.
- The same evidence ID with different content is a hard conflict and the whole binding fails.
- Duplicate or ambiguous subject identities also fail closed.

The operation is atomic: a late conflict cannot leave a partially appended historical context in the live investigation.

## What this does not do

This layer does not read ClickHouse, configure credentials, change production endpoints, alter ARVIS scoring, write ClickHouse data, or replace current evidence. It also does not decide which historical time window a customer-facing handler should request.

That runtime wiring is a separate production step. It must load history before or independently of the current run's memory write, treat history outages as an explicit context gap, and preserve fresh chain evidence as the decision authority.
