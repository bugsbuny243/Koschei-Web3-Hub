# Web3 Bridge Production Webhook Security

Koschei Web3 Bridge is production-hardened for **read-only, no-custody event monitoring**. It does not hold user funds, escrow assets, store private keys, sign transactions, or automatically transfer funds.

## Why unauthenticated webhooks are not production-safe

Unauthenticated webhook endpoints can be called by anyone who knows or guesses the URL. Without verification, an attacker could inject fake transaction events, pollute audit trails, trigger misleading AI summaries, or overwhelm operator workflows. Production webhook ingestion must prove that an event came from a configured source before the event is stored as trusted monitoring data.

## Shared secret header setup

Each event source stores only a SHA-256 hash of its shared secret. The plaintext secret is accepted only during source creation or rotation and is never returned by the API.

Configure the webhook provider to send both headers on every request to `POST /api/web3/events/alchemy`:

- `X-Koschei-Source-Id: <source_id>`
- `X-Koschei-Webhook-Secret: <plaintext shared secret>`

Koschei hashes `X-Koschei-Webhook-Secret` with SHA-256 and compares it to the stored `secret_hash`. If the source is missing, inactive, has no configured secret, or the header is missing/invalid, the webhook is rejected with `401` and is not stored as a trusted event.

## `source_id` usage

Use one source per webhook/provider/network combination where possible. The `source_id` disambiguates sources that share a provider or network and allows secret rotation without relying on payload fields. You can pass it either as:

- the `source_id` query parameter, or
- the preferred `X-Koschei-Source-Id` header.

## Secret rotation

Rotate secrets through `PATCH /api/web3/sources/{id}` with a new plaintext `secret` value. Update the webhook provider header at the same time. Koschei stores only the new SHA-256 hash and never returns the secret hash or plaintext secret from public APIs.

Recommended rotation practice:

1. Generate a new high-entropy secret.
2. Patch the source with the new secret.
3. Update the webhook provider's `X-Koschei-Webhook-Secret` header.
4. Send a test webhook and verify the event is recorded with `verification_status: verified`.

## No private keys, no custody rules

Production Web3 Bridge features must stay inside these boundaries:

- No private keys in Koschei.
- No custody or escrow.
- No user funds held.
- No automatic fund transfers.
- Read-only event monitoring and developer tooling only.
- Human-controlled external wallets remain responsible for all transaction signing and fund movement.

## Railway environment reminder

Alchemy RPC URLs and API keys must stay in Railway environment variables only. Do not commit Alchemy RPC URLs, API keys, webhook secrets, or other provider credentials to the repository or frontend/public files.

## Future Alchemy signature verification roadmap

The shared secret header is the current production-safe baseline. A future hardening step should add official Alchemy webhook signature verification when the provider-specific signature header and signing-secret workflow are available for the configured webhook source. The current Alchemy endpoint keeps a TODO for that provider-specific verification path.
