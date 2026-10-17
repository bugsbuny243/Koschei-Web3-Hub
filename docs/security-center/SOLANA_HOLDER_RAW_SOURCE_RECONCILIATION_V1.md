# Solana holder raw-source reconciliation V1 — observed-only

## Scope and dependency

This additive gate is stacked on **PR #1401** (Solana raw account context slot) → PR #1400 (raw-byte witness). The source reconciliation runs only when a caller supplies **captured raw JSON-RPC response bytes** from these existing read operations:

1. `getTokenSupply` for one explicitly identified Solana mint.
2. `getTokenLargestAccounts` for exactly that mint.
3. `getMultipleAccounts` (`jsonParsed`) for the **ordered token-account addresses** from the largest-account response.

The new `ReconcileHolderRawSources` helper does not issue RPC calls. It is not connected to the current ARVIS verdict engine, owner UI, persistence or a production collection route. It does not change the existing holder scan or risk grading. Legacy and other scanners remain unchanged.

## Fail-closed checks

- Original response bytes <= 2 MiB per source, valid JSON, matching caller-supplied capture SHA-256. Capture digests are **not trusted third-party signatures**.
- Unique and complete `result.context.slot` per source, canonical positive uint64 values; **preserves all three slots independently**.
- `getTokenSupply.value.amount`, largest-account balances and decoded `jsonParsed.info.tokenAmount.amount` must be canonical decimal uint64 strings. No float/UI amount is used. Decimal precision is exactly matched.
- `getTokenLargestAccounts.value` contains 1–20 unique base58 Solana token-account addresses.
- `getMultipleAccounts.value` cardinality and **caller-recorded ordered request addresses** must match largest accounts exactly.
- The sum of bounded largest-token-account raw amounts uses arbitrary-precision integers and must not exceed the separately observed total supply. A violation is rejected as an unresolved cross-read discrepancy, not labeled an exploit. This is **not proof of atomic consistency**.
- Each account's *executable owner* is exactly the SPL Token or Token-2022 program ID. The reported parser type and program must agree. Its parsed mint equals the requested mint. Parsed token amount equals largest-account amount and decimals match supply.
- A parsed `info.owner` is exposed only as `owner_authority`: a pubkey that may be a PDA or multisig, **not proven economic ownership or personal identity**.
- Any inconsistent, stale, malformed or ambiguous input returns **error and zero result**. There is no partial `verified` output.

## Trust boundary

Output `status=observed_non_atomic_source_reconciliation_only` and `SourceWitnesses` contain deterministic hashes and scoped source metadata, never original JSON or RPC endpoint credentials. `atomic_snapshot_verified=false`, `owner_identity_verified=false`, `chain_evidence_verified=false`, `verdict_bound=false` are fixed, even when `same_reported_slot=true`.

Three independent JSON-RPC calls are **not an atomic snapshot**; equal `context.slot` values alone do not make them atomic. The separate calls may see changing balances. Amount mismatches are rejected as **unresolved**, not assumed theft, malicious action or an unsafe verdict. Matching values support only bounded cross-source consistency.

The largest token accounts are **token accounts**, not the underlying wallet/entity holder distribution. A wallet can control multiple token accounts; some authorities are programs, PDAs or multisigs. This checker does not infer `top holders`, beneficial ownership, burn classifications, liquidity control, verified source trust, chain inclusion or exact whole-chain circulation.

### Acceptance

- Fixture regression includes different and equal source slots, preserved token account vs owner authority identities, token-program consistency, Token-2022, maximal uint64 amounts, hash tampering, wrong mint/program/owner, duplicate or non-integer source fields, requested account order and mismatched balances.
- Existing ARVIS scanners and protected JS remain untouched; no new DB migration, RPC method allowlist change, production RPC call, background worker or API exposure.
- Real multi-provider/consensus checks, source-request binding to authenticated provider exchanges, independent economic owner evidence and final EvidenceRows-to-verdict proof **remain not implemented**.

```sh
cd koschei/api
go test ./internal/radaradapter/solana -run 'TestReconcileHolderSources' -count=1 -v
go test ./internal/radaradapter/...
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Merge order: PR #1396 → #1397 → #1398 → #1399 → #1400 → #1401 → this PR. #1395 can be reviewed separately. None of these tests alone constitute a production security certification.
