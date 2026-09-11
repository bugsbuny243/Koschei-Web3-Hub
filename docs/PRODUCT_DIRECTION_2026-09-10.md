# Koschei Web3 product direction — 2026-09-10

Koschei Web3 Hub targets every blockchain network and every address type. ARVIS
owns the common evidence, relationship, attack-path and deterministic decision
layers. Solana/Pump.fun is an existing collection lane; it does not define the
product's maximum scope.

## First implementation slice

- Explicit network selection and offline address resolution under `/fabric/networks`.
- `GET /fabric/networks/catalog` exposes implementation coverage and process-local,
  address-free request/rejection counters.
- `POST /fabric/networks/resolve` accepts JSON or a browser form, binds a subject
  identity to network + address, and never collects evidence or charges credits.
- Solana syntax validation decodes exactly 32 bytes. EVM hexadecimal syntax is
  recognized separately from checksum, ownership and account-existence validation.
- Ethereum, Base, Arbitrum and Optimism are registered with collectors explicitly
  not connected. Bitcoin is registered with parsing and collection pending.
- Unknown/mismatched networks never fall back to the Solana collector.
- Existing scan and ARVIS paths remain in place; the scan console links to coverage.

This slice does not add a live collector, enable continuous workers, satisfy
paid-customer acceptance, or claim complete network coverage. A recognized address
is not a verified account or a security verdict. Current native evidence schemas
and decisions remain unchanged.

## Next acceptance sequence

Current operational blockers and primary-source security/protocol updates are
recorded in [the September 10 Web3 priority watch](WEB3_PRIORITY_WATCH_2026-09-10.md).

1. Add real network-specific evidence adapters behind the common intelligence contract.
2. Bind each adapter's configured network identity, source, block and observation time.
3. Expose collection, freshness, failure and unsupported states in the same UI.
4. Restore durable observation/history/watchlist capabilities through their own
   application persistence path without weakening entitlement checks.
5. Run cross-network collision, missing-evidence and existing-Solana regression cases.

Lang is an independent security programming language. Sentinel is an independent
cybersecurity model. They are sold together in commercial packages; their product
scope exceeds their Web3 integrations. Cross-project access remains contract-only.
See `fabric/product-direction.v1.json` for the recorded owner direction.
