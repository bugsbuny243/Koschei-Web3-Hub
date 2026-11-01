# Binance 29 October 2026: measured historical delegation detection

The existing `ProbeEVM` reads only `latest`. A scan after revocation cannot reveal a prior delegation. `ProbeEVMAtBlock` now reads an explicit hexadecimal block height through the same detector; archive failures never fall back to current state. Historical height and scope survive the intelligence adapter and customer evidence projection. Existing live calls keep their current behavior.

## Captured evidence and result

On 1 November 2026, read-only Alchemy Ethereum mainnet calls retrieved code at four historical heights, block metadata, the delegation receipt and the first sweep transaction. The committed fixture contains exact code results and selected metadata. It contains no credentials, invented transactions or estimated dollar amounts. Block/receipt consistency is checked, but this is a single-provider observation, not independent consensus or Merkle-proof verification.

Wallet: `0x28c6c06298d514db089934071355e5743bf21d60`.

Delegate: `0x63e842b054bd624a465454d533c33f573e9c6fae`.

| Phase | Block | UTC block timestamp | Recorded code | Detector result |
| --- | --- | --- | --- | --- |
| Before delegation | 26300752 | 2026-10-29 20:20:47 | `0x` | No delegation observed |
| Delegation | 26300753 | 2026-10-29 20:20:59 | `0xef0100` + delegate | Delegation observed |
| First large sweep | 26300770 | 2026-10-29 20:24:23 | Same delegation | Delegation observed |
| Revoked | 26301896 | 2026-10-30 00:10:35 | `0x` | No delegation observed |

There are **204 seconds between the delegation block and the first large sweep block**. This is a historical opportunity window, not measured alarm latency or proof that a loss would have been prevented. It starts after delegation was executed; withdrawals on other chains had already begun. End-of-block reads cannot establish instruction ordering within a block.

The test passes archived RPC responses through the production detector and customer projection. It proves the delegation indicator reaches `EIP7702_DELEGATION_OBSERVED` without manufacturing authorization or safety. It is a **state-observation replay**, not EVM execution replay of the attack or verification of all 50 authorization signatures. The original full authorization-list tool response was truncated and is deliberately not represented as a complete capture. The sweep receipt's success status is a selected field, not a full retained log/receipt proof.

## Reproduce

From `koschei/api`:

```sh
go test -v ./internal/networktarget ./internal/services -run '^(TestBinanceArchiveDelegationDetectionWindow|TestHistoricalProbe.*|TestBinanceArchiveReachesCustomerEvidence)$' -count=1
```

The required API CI runs this exact acceptance step. Unit execution uses an explicitly local TLS RPC fixture server; it needs no credentials and broadcasts no transactions. Refreshing the capture requires an archive-capable Ethereum RPC: `eth_getCode(wallet, blockNumber)` and `eth_getBlockByNumber(blockNumber, false)` for each listed height. Compare the header hashes with receipt/transaction block hashes and retain provenance when refreshing.

## Remaining operational gap

No continuous Ethereum treasury subscription, alert delivery deadline, signer/HSM intervention or prevented-loss result is established here. A valid signature or observed delegation alone does not identify an attacker. The next operational acceptance must measure collection, policy, notification and authorized intervention latency against this window, with normal authorized delegations as negative controls. Ordinary cross-chain withdrawals and the compromised DevOps/signing path require separate evidence. Empty code after revocation must never be called proof of an uncompromised history.

## Public references

- [TRM incident investigation](https://www.trmlabs.com/resources/blog/north-korea-steals-usd-6-36-billion-from-binance-in-the-largest-hack-in-history)
- [Delegation transaction](https://etherscan.io/tx/0xad99264d5532ae0dd04b2004eebda05f25657e70dacb764f7ac8d4ccd051298b)
- [First sweep transaction](https://etherscan.io/tx/0x53bd38b816c23e74134786c29a8c18d792777998a3b9326047b583100e88e2f3)
- [Revocation transaction identified in public forensic reporting](https://etherscan.io/tx/0xafbf94c92a8d749988411669fa14d4f8866d9b4420b4ebd299f28e958dac7209)
