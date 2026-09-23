# Global Radar Move Network Event Projection v1

Date: 2026-09-23

## Endpoint

`POST /fabric/radar/network-event`

Request:

```json
{"network":"sui-mainnet"}
```

or:

```json
{"network":"aptos-mainnet"}
```

This endpoint is for network-level identity and freshness evidence that does not require an account/address target.

## Runtime configuration

- `SUI_GRAPHQL_URL` — the HTTPS Sui GraphQL endpoint used for mainnet chain-identifier verification.
- `APTOS_REST_URL` — the HTTPS Aptos REST ledger-index endpoint used for mainnet chain-ID and ledger metadata verification.

No provider secret is committed to the repository.

## Evidence

Sui verifies the canonical mainnet chain identifier before emitting a VERIFIED network-health event.

Aptos verifies `chain_id=1` and preserves bounded ledger metadata such as epoch, ledger version, timestamp, block height and node role.

The normalized probe object is bound with SHA-256 using the same `normalized_probe_json_sha256` source-binding label as the live EVM/Bitcoin projection.

## Boundary

- network identity evidence is not account safety evidence;
- an endpoint being reachable is not a decentralization verdict;
- no wallet location is inferred;
- no ARVIS risk grade or final verdict is created here.
