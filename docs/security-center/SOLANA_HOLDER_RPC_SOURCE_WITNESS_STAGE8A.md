# Solana holder RPC read-source witness — Stage 8A

**Review:** Additive draft, stacked on PR #1401. This patch does not activate any independent on-chain holder-verification or new ARVIS verdict status.

## Existing gap

The established Solana RPC client decoded `getTokenSupply`, `getTokenLargestAccounts` and `getMultipleAccounts` to Go values and discarded their exact JSON-RPC `result` bytes. A later holder analysis could not reproduce its input digest or prove its declared slot came from those original bytes. The existing public response structure must remain unchanged.

## Change

Three existing result types gain a private `Witness SolanaRPCReadWitness` with `json:"-"`. Their new alias-based `UnmarshalJSON` decoders keep original field semantics and silently copy the *exact* raw JSON-RPC result bytes, source `context.slot` (when present), and UTC decode time into the private witness. `SHA256()` commits to the original result bytes, not to a re-marshaled projection.

The standard `solanaRPCDo[T]` already unmarshals its `result` into these types. No RPC call, endpoint, security budget, cache setting or scoring path changes.

### Truth boundaries

- A **missing** `context.slot` remains allowed for legacy consumer compatibility but creates a witness with slot 0; `VerifySource()` and `HolderRPCSourceSet.RequireSameSlot()` reject it.
- Duplicate, null or invalid *present* context slots are rejected; canonical positive uint64 format is required for source readiness.
- To avoid breaking existing valid large RPC reads, a valid JSON result larger than 2 MiB still decodes but **does not produce a witness**. Evidence review consequently fails closed. The existing RPC response size policy is unchanged.
- `HolderRPCSourceSet.RequireSameSlot()` checks four exact raw result witnesses: token supply, largest token accounts, token-account details and owner-wallet account details. It refuses absent, mutated or mismatching slots. **The same reported slot does NOT prove the four calls were atomic, independently checked, or honest provider reports.**
- It does not check the token-account mint, balance arithmetic, owner-wallet address association, Token Program versus Token-2022, token authority, independent provider or chain consensus. The next proof gate must recompute those from captured raw bytes.
- No raw response bytes, endpoint, credential or witness metadata are added to public JSON. No ARVIS/Sentinel `evidence_verified` or grade change, no new incident or security verdict and no autonomous action.

## Tests

```sh
cd koschei/api
go test ./internal/services -run '^TestHolderRPC' -count=1 -v
go test ./... -count=1
go vet ./...
test -z "$(gofmt -l .)"
```

Regressions cover byte-for-byte raw result retention through an RPC envelope, SHA-256, non-leaking JSON serialization, preserving amounts above JavaScript's exact float limit as strings, all three RPC result types, legacy missing-context compatibility, oversized valid response compatibility, duplicate and malformed slots, four-source same-slot requirements and mismatch/tamper rejection.

This is a **source capture** gate; it does not claim completion of the actual holder-concentration EvidenceRows ledger or independently verified chain state. Merge only after parent #1401 is reviewed and merged, with full CI and owner acceptance.
