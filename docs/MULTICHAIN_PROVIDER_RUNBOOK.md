# Multi-chain provider configuration and cost controls

Configure `RADAR_PROVIDERS_JSON` as a server secret. It is a map from the exact network IDs in `CHAIN_CAPABILITY_MATRIX.md` to provider roles. Each role supports `url`, `headers`, `requests_per_second` and `daily_limit`. URLs must be HTTPS and must not contain query credentials, userinfo or fragments. API tokens belong in headers. Operator-controlled URL path keys are never returned or logged.

Example structure (reserved `.invalid` hosts; not working credentials):

```json
{
  "tron-mainnet": {
    "primary": {"url":"https://tron-a.example.invalid","headers":{"TRON-PRO-API-KEY":"REPLACE_IN_SECRET_STORE"},"requests_per_second":1,"daily_limit":10000},
    "secondary": {"url":"https://tron-b.example.invalid","requests_per_second":1,"daily_limit":10000},
    "indexed": {"url":"https://tron-index.example.invalid","requests_per_second":1,"daily_limit":10000}
  },
  "ethereum-mainnet": {
    "primary": {"url":"https://evm-a.example.invalid","requests_per_second":1,"daily_limit":10000},
    "secondary": {"url":"https://evm-b.example.invalid","requests_per_second":1,"daily_limit":10000}
  },
  "ton-mainnet": {
    "primary": {"url":"https://ton.example.invalid","headers":{"X-API-Key":"REPLACE_IN_SECRET_STORE"},"requests_per_second":1,"daily_limit":10000}
  },
  "offchain:dapp.example.invalid": {
    "primary": {"url":"https://dapp.example.invalid","requests_per_second":1,"daily_limit":1000}
  }
}
```

For Tron, primary/secondary must serve the `/wallet` and `/walletsolidity` HTTP APIs. Indexed must serve TronGrid V1. For EVM, the URL is the JSON-RPC endpoint; EIP-1898 canonical block-hash reads are required. For TON, the endpoint must serve both toncenter-compatible `/api/v2` and `/api/v3` paths. TonAPI is not wire-compatible with this adapter and needs a dedicated translation provider; changing the URL alone is insufficient. Cosmos uses an LCD REST endpoint. Stellar uses Stellar JSON-RPC. HTTP redirects are rejected.

Optional pin registries are server-side JSON maps:

- `RADAR_EVM_SWEEPER_HASHES_JSON`: lower-case delegate code Keccak hash -> HTTPS attribution source. Empty registry means no known-sweeper attribution.
- `RADAR_TON_JETTON_MASTERS_JSON`: normalized `workchain:hex` master -> HTTPS canonical source. Required for `jetton_wallet` balance attribution.
- `RADAR_FRONTEND_BASELINES_JSON`: exact `https://host/path` -> SHA-256 hex. Missing baseline means drift unknown.

Frontend `dns`, `certificate`, and `capture` roles are optional. Review `frontend/frontend.go` for their bounded query schemas. User input cannot introduce provider URLs. Two configured hostnames are only a minimum witness condition, not proof that their operators are economically independent.

Each investigation has an 80-request ceiling, 45-second handler timeout, 8-second HTTP timeout and 2 MiB per-response cap. Per-host defaults are deliberately conservative: 1 request/s and 10,000/day **per process**, configurable to match the actual provider agreement. Workers share the existing RPC context budget and cooldown/pacing policy. Multiple service replicas need provider-side or distributed quotas; do not multiply a free tier by replica count. An EVM deep scan can exhaust a 45-second window at 1 RPS; that returns partial evidence rather than an inferred grade.

Published prices and free tiers change. Fixture request counts in the generated matrix measure code paths and do not estimate complete production investigations. Use provider dashboards to set deployment quotas; do not assume the research report's October 2026 price table is a contract. Relevant primary references:

- Tron permission model: https://developers.tron.network/docs/multi-signature
- Tron mainnet genesis identity: https://github.com/tronprotocol/tips/blob/master/tip-474.md
- EIP-7702: https://eips.ethereum.org/EIPS/eip-7702
- EIP-1967: https://eips.ethereum.org/EIPS/eip-1967
- EIP-1898: https://eips.ethereum.org/EIPS/eip-1898
- TON indexed APIs: https://docs.ton.org/api/v3/overview
- Etherscan current limits: https://docs.etherscan.io/resources/rate-limits
- Stellar data APIs: https://developers.stellar.org/docs/data/apis

Read `GET /api/owner/radar/capabilities` with an owner session before attempting live scans. `live_verified: false` stays false because configuration is not a health observation. Live rollout must exercise correct/wrong-network cases and provider failure without exposing provider secrets. All investigations remain observe-only and return WITHHOLD.

## Repeatable smoke gate

From `koschei/api`, `go run ./cmd/radar-smoke` prints configuration status only, without network calls. For live reads, supply an explicit manifest and `-live`:

```json
[
  {
    "target": {"network":"ethereum-mainnet","kind":"wallet","target":"0x0000000000000000000000000000000000000000"},
    "required_relations": ["evm-code"]
  }
]
```

The zero-address example checks a minimal code-read path only; it is not a known-sweeper or proxy acceptance case. Choose public targets and required relations for each actual rollout scenario. Use `tron-permission-state`, `ton-account-state` or `ton-jetton-wallet-binding` for their respective target classes.

```
go run ./cmd/radar-smoke -live -manifest /secure/operator-smoke-cases.json
```

Exit 0 requires the expected relations, correct subject/network binding, source, observation time, chain identity and response hashes while keeping `grade_issued=false` and `verdict=withhold`. Exit 1 means the requested collection assertions failed; exit 2 means an invalid manifest. At most ten cases run sequentially with a 45-second timeout each. The output is a compact provenance receipt, not provider credentials or raw upstream content. Store receipts with your deployment evidence; a passing smoke covers only its asserted relations.

On 2026-10-14, configuration inspection in the development runtime found no `RADAR_PROVIDERS_JSON`. No live-provider success is claimed. The tool's negative-path test proves missing evidence cannot pass.

Tron V1 token-history reference: https://developers.tron.network/reference/get-trc20-transaction-info-by-account-address . The collector caps the result at 50 events, validates account/token identities and preserves integer values as strings. The endpoint may contain TRC-721 and approval events too; it does not prove a fungible standard or current allowance. A confirmed-head context is labeled separately from an event's unknown block height.
