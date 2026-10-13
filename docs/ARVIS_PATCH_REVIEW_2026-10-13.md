# ARVIS evidence-contract patch follow-up

This change adapts the two supplied patches (`468cb918` and `47cb4f60`) to
`fix/arvis-evidence-contract-closeout-20261013` at `05a477d1`. Those supplied
commit objects were not present in the fetched remote refs. The base already
requires the shared Solana RPC manager for LP reads; its fail-closed behavior
is preserved, with no direct HTTP fallback reintroduced.

## Changes and regression coverage

| Area | Result | Regression coverage |
| --- | --- | --- |
| Tron permission JSON | Numeric and `Owner` / `Witness` / `Active` enum forms parse; unknown values and mismatched permission containers fail. Omitted zero-valued Owner remains supported for protobuf JSON. | `TestTronPermissionEnumWireForms` |
| Tron evidence identity | IDs include canonical account, full response digest, height, UTC observation time and permission ID. Identical content observed at different times is distinct; replaying one observation is idempotent. | `TestTronEvidenceIDsSeparateObservationEvents` |
| Subject binding | Tron family, chain and classification basis accompany the canonical subject; bridge output retains family and chain. | `TestTronWalletRegistryEndToEndObservedOnly` |
| HTTP contract | Success has `ok`, `withhold`, unmet evidence reasons and explicit observation limitations. Error and success responses include collection status, provider, collection latency and row count. One sanitized operator log is emitted per request. | `TestTronWalletEvidenceHTTPContract` |
| OpenAPI | Required address query parameter and provider-unavailable 503 response are declared; removed synthetic endpoints are absent. | `TestTronRouteDeclaresRequiredAddressAndProviderFailure`, generated-document drift test |
| Synthetic routes | MEV, liquidity and DAO request-scoring endpoints are removed. Six existing handler files are quarantined; files and database rows are preserved. | AST route guards and existing product-route tier test |
| Public impact | All aggregate loss amounts, rolling 24-hour / 7-day amounts, biggest prevention, inferred prevention counts and per-event amounts/scores are withheld from JSON. Corresponding calculations are no longer queried. Cache namespace is bumped. Counts remain record counts. | `TestPublicImpactCannotPublishUnverifiedAmounts`, `impact-contract.test.cjs` |
| Solana transaction version | Settlement uses the shared supported-version constant; literal guard rejects unequal values in either direction. | Existing transaction-version guard |
| Token program display | Canonical classic SPL and Token-2022 program IDs are recognized explicitly. Missing, unknown, malformed or conflicting evidence cannot become a classic VERIFIED claim. | Seven Node regression cases |

The initial regression run failed on the pre-fix code for enum strings,
colliding IDs, missing query parameters, missing HTTP fields, detached chain
identity and exposed impact claims. The initial Node run also failed for all
seven program-identity cases. Fixtures are synthetic and no live-provider
success is asserted.

## Scope and compatibility

This is an evidence-contract correction, not completion of the multi-chain
roadmap. Tron remains single-provider, observed-only and unanchored to account
state at a specific block. This patch does not add failover, TRC-20 coverage,
EVM-depth collectors, TON traces, Cosmos or Stellar execution.

The public impact API intentionally stops returning fields that previously
claimed unverified financial prevention. Legacy Go fields remain for source
compatibility with JSON serialization disabled; the bundled public UI no longer
coerces their absence to zero or labels record counts as prevented attacks.
Consumers outside this repository must likewise tolerate absent metrics.

No merge, deployment, paid provider provisioning or database deletion is part
of this change. CI and review remain required before integration.
