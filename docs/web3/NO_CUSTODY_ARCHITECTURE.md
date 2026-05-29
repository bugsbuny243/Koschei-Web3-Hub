# Koschei Web3 Bridge No-Custody Architecture

Koschei Web3 Bridge is an MVP foundation for read-only Web3 event monitoring and developer tooling. It is designed for grant and demo credibility without taking control of user assets.

## Safety boundaries

- **No private keys:** Koschei does not ask for, store, generate, encrypt, decrypt, or transmit user wallet private keys or seed phrases.
- **No custody:** Koschei does not hold user funds, tokens, NFTs, or signing authority.
- **No escrow:** Koschei does not intermediate trades, lock assets, or operate escrow accounts/contracts in this MVP.
- **No automatic transfers:** Koschei does not initiate on-chain transfers, swaps, mints, burns, approvals, staking actions, or withdrawals.
- **Human approval for high-impact actions:** Any future action that could affect funds, permissions, publishing state, or production infrastructure must be explicitly approved by a human before execution.

## What the MVP does

The MVP adds webhook/event monitoring tables and API routes that record Web3 events from external providers such as Alchemy. The backend stores the raw JSON payload and best-effort fields such as network, event type, transaction hash, wallet address, and contract address.

Current capabilities:

1. Receive an Alchemy-style webhook payload at `POST /api/web3/events/alchemy`.
2. Store the payload in `web3_events` for audit and demo review.
3. Create a protected test event for demos at `POST /api/web3/events/test`.
4. List the latest protected user events at `GET /api/web3/events`.
5. Generate and persist a deterministic plain-language explanation at `POST /api/web3/events/{id}/explain`.
6. Display recent events in the static Web3 Bridge dashboard concept.

## What the MVP does not do

The MVP intentionally does **not** include:

- Wallet key management.
- Custodial wallets.
- Escrow logic.
- Smart contract deployment.
- Transaction signing.
- Automatic transaction submission.
- Token transfer automation.
- Background workers for fund movement.

## Webhook security roadmap

The current unauthenticated webhook endpoint is for early integration and demo setup only. Before production use, Koschei should add provider-specific webhook signature verification.

Planned hardening steps:

1. Configure a per-source webhook signing secret in a secrets manager or provider dashboard.
2. Verify the Alchemy signature header before reading the payload as trusted input.
3. Reject unsigned or invalid webhook requests with `401 Unauthorized`.
4. Store signature verification status in event metadata or alert records.
5. Add replay protection if the provider exposes timestamp/nonce headers.
6. Add per-source rate limits and provider allowlists.

## Future Solana and Alchemy setup guide

A future setup guide should document:

1. Creating an Alchemy app for the target network, starting with `solana-devnet` for demos.
2. Creating a webhook in Alchemy that points to `POST /api/web3/events/alchemy`.
3. Choosing monitored wallet or contract addresses.
4. Mapping Alchemy payload fields into Koschei `web3_events` fields.
5. Enabling webhook signature verification.
6. Testing with a devnet transaction and confirming the dashboard shows the event.

## Operating principle

Koschei Web3 Bridge observes and explains activity. It does not control assets. Users keep signing authority in their own wallets and external systems.
