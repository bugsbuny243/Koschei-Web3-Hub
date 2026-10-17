# Solana account response context.slot integrity — V1

**Status:** Additive draft, stacked on PR #1400. No production claims until CI and reviewer acceptance.

## Gap addressed

The existing `ReadAccountAtSlot` used `getSlot` as a finalized lower bound, sent it as `minContextSlot` to `getAccountInfo`, and checked the result context was not older. However, the Solana program adapter wrote that original **lower-bound slot** into the observed `EvidenceRow.Height`, even when the account response reported a later slot.

The new `accountResponseContextSlot` reads the **original raw `getAccountInfo` JSON response**, extracts the exact `result.context.slot`, and requires a canonical positive uint64 not older than the separately obtained finalized lower bound.

It rejects missing/duplicate `result`, `context` or `slot` keys, malformed JSON, floating/exponent/string slots, negative/zero/overflow values and stale responses. It applies the same 2 MiB source JSON bound as the raw-byte witness primitive from #1400. It does not reconstitute JSON from a previously parsed value.

## Collector binding

When the response is valid, the preexisting Solana upgradeable-program adapter:
- uses **response context slot** as `EvidenceRow.Height` and the existing `Attributes["height"]`;
- keeps the earlier `getSlot` result as `requested_min_context_slot`;
- attaches `account_context_slot` and `account_context_scope=raw_rpc_context_slot_not_consensus_or_atomic_snapshot`;
- passes the exact same raw account response bytes into `BindRawJSONSourceWitnesses` for SHA-256 checking;
- returns no evidence and a `solana_account_context_slot_unbound` limitation when the raw context cannot be uniquely bound.

This is source-consistency verification, **not** provider-independent slot validation, chain inclusion, validator consensus, proof of program authority, or an atomic snapshot. A finalized `getSlot` request and a later `getAccountInfo` request can legitimately report different slots.

## Continuity

No RPC calls added. No modifications to the existing `ReadAccountAtSlot` signature, main ARVIS grade rules, Sentinel runtime, authorization, owner incident workflow, database schema or protected UI files. The new code stays within `internal/radaradapter/solana`.

## Acceptance

Unit cases cover exact/newer/stale slots, absent or duplicate JSON context fields, malformed or non-integer values and overflows. A simulated two-call Solana collector flow demonstrates `getSlot=42` / `getAccountInfo.context.slot=43` produces an **observed** row with height 43, no extra RPC call, and a preserved `solana_non_atomic_reads` limitation.

Run full Go 1.26.9, PostgreSQL 17, security CI, CodeQL and the source witness tests; do not merge ahead of #1400.

```sh
cd koschei/api
go test ./internal/radaradapter/solana -run 'Test(AccountResponseContextSlotExactRawJSON|SolanaCollectorUsesAccountResponseSlotNotLowerBound)' -count=1 -v
go test ./internal/radaradapter/...
gofmt -l internal/radaradapter/solana/account_context_witness*.go internal/radaradapter/solana/adapter.go
```

**Next gate:** independently parse and reconcile Solana token supply, largest token accounts and token-account owners from their *respective* raw response bytes, with exact integer amounts and explicitly non-atomic slot tracking. Do not promote the resulting rows to verified until those source-specific checks and an authenticated verdict link are present.
