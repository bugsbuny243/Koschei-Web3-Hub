# Koschei Web3 Repository Baseline — 2026-09-12

This document records repository evidence only. It is not production deployment proof.

Baseline commit: `19c7dcfa4308ec8711434c811790695ea9c939b0`
Previous audit commit: `b972ffe5276890e604af069c2bc9575be39a6e45`
Delta: 87 commits ahead.

## Rules

- Code presence is not deployment proof.
- `probe_ready` is not full collector coverage.
- Missing integrations remain missing until repository evidence and acceptance evidence exist.
- Research documents do not override repository evidence.
- Completion requires executed acceptance evidence, not only files or tests being present.

## Network capability baseline

| Network | Family | Repository state |
| --- | --- | --- |
| Solana mainnet | solana | `existing` |
| Ethereum mainnet | evm | `probe_ready` |
| Base mainnet | evm | `probe_ready` |
| Arbitrum mainnet | evm | `probe_ready` |
| Optimism mainnet | evm | `probe_ready` |
| Bitcoin mainnet | utxo | `probe_ready` |

`probe_ready` means the repository contains the probe path and related handling. It does not mean complete historical collection, production provider health, or full network coverage has been proven.

## Persistence baseline

The Web3 runtime remains stateless by default, but repository support now exists for durable application persistence through `APP_DATABASE_URL` and optional read persistence through `APP_DATABASE_READ_URL`.

Paid-access enforcement has a separate entitlement store through `ENTITLEMENT_DATABASE_URL`. When it is absent, paid customer operations remain fail-closed.

Whether those environment variables are configured in the live deployment is outside this repository baseline and remains `unknown` until deployment evidence is checked.

## Agent evidence baseline

Repository evidence exists for:

- identity/delegation evidence;
- execution evidence;
- material-action binding;
- execution trace validation.

No repository integration was verified for MCP, A2A, WebMCP, or AP2 at this baseline.

## Payments baseline

An x402 pay-per-tool foundation exists and is disabled by default. Repository presence is not proof of a real x402 payment integration or money movement.

## Important changes since the 2026-09-11 audit

- auth redirect rejects backslash/CR/LF in frontend redirect values;
- JWKS hardening regression tests were added;
- application persistence support was added;
- Bitcoin address parsing and probe support were added;
- EVM probe support was added;
- agent material-action binding was added;
- the legacy Neon auth cleanup helper was removed;
- ClickHouse ARVIS read-index migration support was added.

## Not proven in this repository baseline

- C2PA provenance integration;
- ML-KEM provider;
- ML-DSA provider;
- production Verifiable Credentials / DID integration;
- physical-device authority runtime;
- neural/biometric data runtime;
- complete autonomous revocation/stop proof.

This file is the repository-truth baseline for later planning documents. Research packages may extend it, but they must not silently promote research or planned work to implemented state.
