# Solana holder raw-source consistency gate — V1

State: Draft, opt-in and read-only. This PR is stacked on #1401. It does NOT alter production ARVIS verdicts, holder grades, existing RPC managers, customer APIs, or incident controls.

## Inputs
CheckHolderRawSources requires an explicit supported network identity (solana-mainnet), a validated requested mint, and three ORIGINAL JSON-RPC response byte buffers from that request:
- getTokenSupply (exact amount, decimals, reported context.slot)
- getTokenLargestAccounts (up to 20 token-account addresses and exact raw amounts)
- getMultipleAccounts with jsonParsed encoding, for the exact ORDERED list of token accounts in the second response.

The caller must independently provide the ordered getMultipleAccounts request arguments, not recreate them from response values. This module does not perform network requests. The existing live holder path currently consumes decoded responses; wiring original reply bytes into that path remains separate future work.

## Fail-closed checks
Three response envelopes are valid JSON, bounded at 2 MiB apiece, contain a unique result, and do not contain a non-null RPC error. Unique result.context.slot fields must be canonical positive integers. Checked critical nested fields use unique-key extraction. Mint and token-account public keys must parse as Solana public keys. Raw amounts are canonical uint64 decimal strings with NO floats, exponents, negatives or overflow. Decimals must agree throughout.

Largest accounts must be sorted by descending raw balance, unique and limited to 20. Ordered getMultipleAccounts return count must match the requested account order; missing accounts or conflicting token mints are rejected. Each resolved account must belong to the pinned SPL Token or Token-2022 program and be parsed as a token account; controlling owner pubkey is reported separately from the token account pubkey and token program ID. Top balances use arbitrary-precision summation.

Only when all THREE reported slots are equal do account balances have to match and top sum have to fit within total supply. If slots differ, both raw balances remain separate; status is observed_non_atomic_slot_set, with NO supply/owner reconciliation or concentration verdict.

## Verification boundary
The output contains original full-response SHA-256 hashes, individual slots, exact integer amounts and the separate controlling owner pubkeys. It contains no raw provider payload, token or RPC URL.

ChainEvidenceVerified=false, VerdictBound=false and AtomicSnapshotProven=false are fixed. Even matching reported slots do NOT establish an atomic snapshot, provider authenticity, independent Solana consensus or source-to-ARVIS signed verdict linkage. A missing/incomplete source returns error and no projection — never a SAFE/LOW interpretation.

## Existing system preserved
No files deleted. No changes to existing holder scoring, ARVIS grade, Sentinel workers, owner endpoints, authentication, DB migrations or outbound allowlists. Unit regressions use synthetic local RPC JSON, not live RPC verification. Merge dependency: #1396 -> #1397 -> #1398 -> #1399 -> #1400 -> #1401 -> this PR.
