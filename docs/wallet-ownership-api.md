# Wallet Ownership API

Koschei links a Solana wallet to an authenticated account by verifying a signed message. The flow never requests a seed phrase or private key and never authorizes a transaction.

## Flow

1. `POST /api/auth/wallet/challenge`
   - Requires the normal Koschei bearer token.
   - Body: `{ "wallet_address": "...", "network": "solana-mainnet" }`
   - Returns a short-lived message and challenge ID.

2. The wallet signs the exact UTF-8 message with `signMessage`.

3. `POST /api/auth/wallet/verify`
   - Body: `{ "challenge_id": "...", "signature": "<base64-or-base58>" }`
   - The Go API decodes the Solana public key and verifies the Ed25519 signature.
   - The challenge is single-use and expires after five minutes.

4. `GET /api/auth/wallet/status` returns the active verified link.

5. `POST /api/auth/wallet/unlink` revokes the active link and clears the profile wallet address.

## Security properties

- Challenge ownership is bound to the authenticated subject.
- Challenges expire and cannot be replayed after successful use.
- A wallet cannot be actively linked to two accounts.
- Linking does not grant token-tier access by itself; balance and tier evaluation are separate read-only steps.
- No transaction, transfer, approval or token authority is requested.
