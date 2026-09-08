# ClickHouse publication ledger v1

Status: additive validation layer; no live publication cutover.

## Purpose

The legacy PostgreSQL publication path couples mutable current state with
transaction triggers and immutable audit rows. ClickHouse is not used as an OLTP
replacement for those locks/triggers. This layer stores immutable facts and derives
visibility only from a fully verified transition chain.

```text
immutable dossier artifact
        |
        | SHA-256 + safe artifact URI
        v
dossier_bundle_manifests
        |
        v
publish -> update/feature -> hide -> republish
        |
        | previous transition UUID + SHA-256
        v
dossier_publication_transitions
        |
        v
verified current-state projection
```

## Fail-closed rules

- `case_ref` keeps the existing `KD1-...` identity contract.
- chain-native target IDs are never lowercased.
- a manifest ID is deterministic from `case_ref + bundle_sha256`.
- the full manifest has a separate SHA-256 receipt, so conflicting metadata for
  the same immutable bundle remains detectable.
- transition IDs and hashes are deterministic from canonical transition fields.
- sequence gaps, repeated sequence forks, predecessor mismatches, payload tamper,
  actor/publisher mismatch, and invalid exposure-time semantics invalidate the
  chain.
- `featured=true` is valid only for `status=public`.
- evidence artifact URIs may not carry credentials, query strings, or fragments.
- raw immutable dossier bytes remain in the evidence/object-storage boundary;
  ClickHouse stores provenance, hashes, typed facts, and the publication ledger.

## Effective time

`exposure_started_at` mirrors the existing publication-time security semantics:

- first publish starts an exposure interval,
- updates/features while public preserve the interval start,
- hide/draft preserve the previous interval start as historical provenance,
- republish starts a new interval.

This prevents artifact creation time from being confused with public exposure time.

## Concurrency boundary

MergeTree deliberately does not hide conflicting writes. If two writers append
sequence `N` for the same case with different transition hashes, both remain
observable and the projection must fail closed. Before live writes, Koschei must
have one serialized publication-writer authority (or an equivalent external claim
contract) plus readback verification.

## Deployment gates

Do not wire customer/public reads or owner publication writes to this ledger until:

1. migration 002 passes ClickHouse 26.2 CI,
2. manifest-conflict and same-sequence-fork detection are proven,
3. bounded ClickHouse readback validates the complete chain before projection,
4. the writer performs append -> readback -> hash-chain verification,
5. publish/hide/republish and failure-injection tests pass,
6. real immutable dossier artifacts have verified URI + SHA-256 provenance,
7. production credentials live only in the deployment secret store.

## Infrastructure boundary

This work must not change the ClickHouse Cloud plan, tier, TDE setting, replica
count, replica size, RAM/vCPU capacity, or billing configuration. It does not
modify Koschei Sentinel or Koschei Lang.
