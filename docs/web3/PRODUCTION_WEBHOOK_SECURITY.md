# Web3 Bridge Production Webhook Security

Koschei Web3 Bridge is production-hardened for **read-only, no-custody event monitoring**. It does not hold user funds, escrow assets, store private keys, sign transactions, or automatically transfer funds.

## Why unauthenticated webhooks are not production-safe

Unauthenticated webhook endpoints can be called by anyone who knows or guesses the URL. Without verification, an attacker could inject fake transaction events, pollute audit trails, trigger misleading AI summaries, or overwhelm operator workflows. Production webhook ingestion must prove that an event came from a configured source before the event is stored as trusted monitoring data.


## Dashboard authentication

The `/web3-bridge.html` dashboard is for authenticated Koschei app users. Source creation, source listing, recent event listing, test event creation, and event explanation all call protected API routes and require a valid Neon Auth-backed application session. If the browser does not have an app session available, the dashboard must show: `Please sign in first to manage Web3 event sources.`

Production users should use the normal app login flow before managing Web3 event sources. Manual bearer-token entry is temporary admin/testing access only, should be kept collapsed away from the main dashboard flow, and must not be used as the normal production authentication model.

Current implementation note: custom `/api/auth/register` and `/api/auth/login` are disabled, and the backend only verifies Neon Auth bearer sessions through protected routes such as `/api/me`. `/login.html` is therefore a safe static placeholder that can detect an already-created browser session, redirect authenticated users to `/web3-bridge.html`, and clear local session storage, but it does not mint tokens or fake authentication. Wire the production Neon Auth frontend flow into `/login.html` before relying on it as the normal user sign-in page.

## Alchemy Auth Token setup

Alchemy webhook setup can use its Auth Token flow, which avoids relying on custom headers that may not be available in the Alchemy UI.

Use this webhook URL pattern:

```text
https://tradepigloball.co/api/web3/events/alchemy?source_id=SOURCE_ID
```

Then set the Alchemy **Auth Token** to the private webhook secret generated in Neon/Koschei for that event source.

Security notes:

- Do **not** put the webhook secret in the URL.
- `source_id` is not secret; it identifies which configured `web3_event_sources` row should be used for verification.
- `webhook_secret` is secret; treat it like a password and store it only in the provider's Auth Token or supported secret-header field.
- Koschei stores only a SHA-256 hash of the webhook secret and never returns the plaintext secret or stored `secret_hash` from public APIs.

## Supported verification inputs

Each event source stores only a SHA-256 hash of its shared secret. The plaintext secret is accepted only during source creation or rotation and is never returned by the API.

Koschei looks up the active Alchemy source by `source_id`, which can be passed as either:

- `source_id=SOURCE_ID` on the webhook URL query string, or
- `X-Koschei-Source-Id: SOURCE_ID` when the provider supports custom headers.

After the source is found, Koschei accepts the webhook secret from the first supported location present:

- `X-Koschei-Webhook-Secret: <plaintext shared secret>`
- `Authorization: Bearer <plaintext shared secret>`
- `X-Alchemy-Token: <plaintext shared secret>`

Koschei hashes whichever secret was provided with SHA-256 and compares it to the stored `secret_hash`. If the source is missing, inactive, has no configured secret, or the provided secret is missing/invalid, the webhook is rejected with `401` and is not stored as a trusted event. Koschei must not log the plaintext secret, return the stored `secret_hash`, or disclose the expected secret in error responses.

## Custom header setup

If a webhook provider supports custom headers, you can still configure the legacy header flow on every request to `POST /api/web3/events/alchemy`:

- `X-Koschei-Source-Id: <source_id>`
- `X-Koschei-Webhook-Secret: <plaintext shared secret>`

This remains supported for providers and tools that can send custom headers. For Alchemy, prefer the Auth Token setup above when custom headers are unavailable.

## Secret rotation

Rotate secrets through `PATCH /api/web3/sources/{id}` with a new plaintext `secret` value. Update the webhook provider Auth Token or secret header at the same time. Koschei stores only the new SHA-256 hash and never returns the secret hash or plaintext secret from public APIs.

Recommended rotation practice:

1. Generate a new high-entropy secret.
2. Patch the source with the new secret.
3. Update the webhook provider's Auth Token, `Authorization: Bearer <secret>`, `X-Alchemy-Token`, or `X-Koschei-Webhook-Secret` value.
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

The shared secret/Auth Token flow is the current production-safe baseline. A future hardening step should add official Alchemy webhook signature verification when the provider-specific signature header and signing-secret workflow are available for the configured webhook source. The current Alchemy endpoint keeps a TODO for that provider-specific verification path.
