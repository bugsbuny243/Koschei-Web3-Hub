# Web3 Bridge Production Webhook Security

Koschei Web3 Bridge is production-hardened for **read-only, no-custody event monitoring**. It does not hold user funds, escrow assets, store private keys, sign transactions, or automatically transfer funds.

## Why unauthenticated webhooks are not production-safe

Unauthenticated webhook endpoints can be called by anyone who knows or guesses the URL. Without verification, an attacker could inject fake transaction events, pollute audit trails, trigger misleading AI summaries, or overwhelm operator workflows. Production webhook ingestion must prove that an event came from a configured source before the event is stored as trusted monitoring data.


## Dashboard authentication

The `/web3-bridge.html` dashboard is for authenticated Koschei app users. Source creation, source listing, recent event listing, test event creation, and event explanation all call protected API routes and require a valid Neon Auth-backed application session. If the browser does not have an app session available, the dashboard must show: `Please sign in first to manage Web3 event sources.`

Production users authenticate through the normal Koschei login page at `/login.html` before managing Web3 event sources. The login flow uses the current Neon Auth / Better Auth email + password provider configuration: users enter their email address and provider password, Koschei forwards those credentials to the provider-backed `/sign-in/email` endpoint, and the browser stores only the provider-issued bearer JWT in the same app session storage keys already read by `/web3-bridge.html`. Koschei does not store, log, or return passwords; it does not implement custom password authentication and does not mint local replacement tokens.

Web3 Bridge APIs require verified bearer/session auth. Protected routes such as `/api/me`, `/api/web3/sources`, `/api/web3/events`, `/api/web3/events/test`, and event explanation verify the Neon Auth / Better Auth JWT with `NEON_AUTH_JWKS_URL` and `NEON_AUTH_ISSUER` before they create source rows, list events, or perform source/event actions. `/api/me` remains the source of truth for the browser session and upserts `app_user_profiles` by auth subject and email only after token verification succeeds. If a bearer/session token is missing, expired, signed by an unknown JWKS key, has an invalid issuer/audience, or lacks the required `sub` and `email` claims, the APIs must return `401` and remain locked.

Manual bearer-token entry in `/web3-bridge.html` remains admin/testing only. It is useful for controlled verification while operating or debugging the deployment, but it is not the production user authentication model and should not be treated as a replacement for normal Koschei login.

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

Production auth uses the current Neon Auth / Better Auth dashboard configuration with email + password sign-up/sign-in. Railway must provide these auth environment variables:

- `NEON_AUTH_BASE_URL`
- `NEON_AUTH_ISSUER`
- `NEON_AUTH_JWKS_URL`
- `NEON_AUTH_AUDIENCE` (optional; omit unless known)
- `EXPO_PUBLIC_NEON_AUTH_URL`

`NEON_AUTH_BASE_URL` must be the Auth URL copied from the Neon Auth configuration and must expose `/sign-in/email`; Koschei also uses that same base for safe provider follow-up checks at `/token`, `/get-session`, and `/session` when sign-in returns an opaque session token instead of a bearer JWT. `NEON_AUTH_ISSUER` must match the provider bearer JWT `iss` claim. `NEON_AUTH_JWKS_URL` must match the provider signing keys used to verify those bearer JWTs. `NEON_AUTH_AUDIENCE` is optional and should be omitted unless the expected JWT audience is known. `EXPO_PUBLIC_NEON_AUTH_URL` is for static/frontend hints only; protected APIs verify provider-issued bearer JWTs with the backend auth variables. Stack Auth-style project ID and publishable-client-key variables are not required for this production login flow. Email OTP is not part of this auth mode unless the Neon Auth / Better Auth OTP plugin is enabled later and the backend is intentionally changed to call the OTP endpoints.

Alchemy RPC URLs and API keys must stay in Railway environment variables only. Do not commit Alchemy RPC URLs, API keys, webhook secrets, or other provider credentials to the repository or frontend/public files.

## Future Alchemy signature verification roadmap

The shared secret/Auth Token flow is the current production-safe baseline. A future hardening step should add official Alchemy webhook signature verification when the provider-specific signature header and signing-secret workflow are available for the configured webhook source. The current Alchemy endpoint keeps a TODO for that provider-specific verification path.
