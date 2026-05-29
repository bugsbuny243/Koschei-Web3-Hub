# Solana Integration Production Guide

This guide documents the production boundaries and rollout path for Solana support in Koschei Web3 Bridge.

## Official Solana documentation

Use the official Solana documentation as the primary reference for protocol behavior, RPC methods, transaction concepts, and network guidance:

- [Official Solana documentation](https://solana.com/tr/docs)

## Production safety boundaries

Koschei Web3 Bridge uses Solana in **read-only mode first**. The initial Solana integration is for monitoring, ingesting, storing, and explaining events. It must not control wallets or move assets.

The Solana integration must keep these boundaries:

- **No private keys:** Do not ask for, store, generate, encrypt, decrypt, log, or transmit private keys, seed phrases, keypairs, or signing material.
- **No custody:** Do not hold user SOL, SPL tokens, NFTs, wallet authority, or any other assets.
- **No escrow:** Do not lock assets, intermediate trades, or operate escrow contracts/accounts.
- **No automatic transfers:** Do not initiate, sign, submit, or schedule transfers, swaps, mints, burns, approvals, staking actions, withdrawals, or other asset-moving transactions.
- **Read-only first:** Treat Solana support as event monitoring and developer tooling until a separately reviewed human-approved design expands the scope.

## RPC and secret handling

Alchemy Solana RPC URLs must stay in Railway environment variables only. Never commit Alchemy Solana RPC URLs, API keys, webhook secrets, or other provider credentials into GitHub, source files, frontend assets, docs examples, screenshots, logs, or test fixtures.

Documentation and examples may refer to placeholder environment variable names, but must not include real Alchemy URLs or keys.

Recommended environment-only placeholders:

- `ALCHEMY_SOLANA_DEVNET_RPC_URL`
- `ALCHEMY_SOLANA_MAINNET_RPC_URL`
- `WEB3_ALCHEMY_WEBHOOK_SECRET`

Do not touch Railway config as part of documentation-only Solana guide changes unless a separate infrastructure task explicitly requires it.

## Network rollout: devnet first, mainnet later

Use Solana devnet first for integration and demo validation. Mainnet support should come later only after read-only ingestion, webhook verification, event parsing, logging, and dashboard behavior are reviewed in a production-readiness pass.

Suggested rollout sequence:

1. Configure a Solana devnet Alchemy app and keep its RPC URL in Railway ENV only.
2. Register a devnet webhook source in Koschei with a source-scoped shared secret.
3. Send devnet webhook events to Koschei and verify they are stored as read-only monitoring events.
4. Confirm dashboard and event explanation behavior without exposing secrets.
5. Review production readiness before enabling Solana mainnet monitoring.
6. Enable mainnet read-only monitoring only after the same secret, logging, and no-custody boundaries are confirmed.

## Webhook ingestion

Solana webhook ingestion should flow through the existing Koschei Alchemy ingestion endpoint:

```text
POST /api/web3/events/alchemy
```

Production webhook requests must include both Koschei verification headers:

```text
X-Koschei-Source-Id: <source_id>
X-Koschei-Webhook-Secret: <plaintext shared secret>
```

The source ID should identify the configured provider/network source. The webhook secret should match the source-scoped secret stored by Koschei as a hash. Missing or invalid production headers should cause the webhook to be rejected rather than stored as trusted monitoring data.

## Transaction and event lookup roadmap

Future Solana lookup work should remain read-only and should prioritize operator visibility, auditability, and event explanation quality.

Roadmap items:

1. Add transaction signature lookup for stored Solana events.
2. Normalize Solana event fields such as network, transaction signature, slot, block time, wallet address, program ID, token mint, amount, and event type where available.
3. Add read-only wallet activity lookup for configured demo addresses.
4. Link stored webhook payloads to follow-up RPC lookups for clearer event summaries.
5. Add dashboard filters for Solana network, source ID, transaction signature, wallet, program ID, and verification status.
6. Preserve raw provider payloads for audit review while exposing normalized fields for search and explanations.

## Future Solana RPC and event parsing improvements

Future Solana work should improve official RPC usage and provider event parsing without changing the no-custody posture.

Planned improvements:

- Track official Solana RPC documentation updates and align parsing with canonical transaction and account data structures.
- Improve parsing for SPL token transfers, native SOL transfers, program invocations, account changes, and token balance changes.
- Add provider-specific parsing adapters that can evolve without changing the stored audit payload.
- Add explicit verification metadata for source ID, shared secret status, network, and provider.
- Add clearer error reporting for unsupported Solana event shapes.
- Keep all RPC/event parsing paths read-only unless a separate reviewed design explicitly authorizes a broader scope.

## Operating principle

Koschei Web3 Bridge observes Solana activity and explains it. It does not custody assets, store private keys, operate escrow, or automatically transfer funds.
