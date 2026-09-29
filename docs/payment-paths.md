# Koschei payment path

## Supported billing boundary

Koschei Web3 has one paid customer plan: **Professional**. Paid customer access is granted only through an active server-side Professional entitlement.

The repository does not expose a browser-controlled price or a client-side entitlement activation path. Payment-provider data is treated as external evidence for an entitlement decision, not as authority supplied by the frontend.

## Supported provider records

Polar is the active hosted billing edge for Koschei Web3. Existing `shopier`, `shopier_manual`, and `owner_manual` entitlement records remain supported for already-authorized/manual operations. Provider-specific tokens, webhook secrets and Polar product IDs remain server-side and must never be embedded in the frontend bundle or committed to the repository.

Any provider identifier outside the explicit allowlist is rejected. Retired Paddle evidence is not accepted for new activation and must never be silently normalized to an owner/manual path.

## Polar authorization flow

1. An authenticated customer requests checkout for the canonical `professional` plan.
2. The backend maps `professional` to the server-configured Polar product ID and creates a hosted Polar checkout. The browser receives only the hosted checkout ID/URL.
3. Checkout creation or redirect success **does not grant product access**.
4. Polar sends a signed webhook to `/api/polar/webhook`.
5. The backend verifies the raw webhook body and signed delivery headers before parsing or trusting event data.
6. For `subscription.active`, the backend verifies the product-to-plan mapping and the authenticated-subject/email metadata binding, records an idempotent evidence digest, and activates the server-side Professional entitlement.
7. `subscription.canceled` and `subscription.past_due` are recorded but do not immediately revoke Koschei access; Polar can keep paid-period/grace-period access alive in those states.
8. A verified `order.paid` with `billing_reason=subscription_cycle` and an active, correctly bound subscription refreshes that exact Polar entitlement period and restores Professional output capacity. Pending `order.created`, unpaid orders, purchases and proration orders do not refresh quota.
9. For `subscription.revoked`, only the entitlement carrying the exact `polar` provider plus subscription ID evidence is revoked. Other manual/provider grants are not touched, and the customer profile is recomputed from any remaining active entitlement.
10. Duplicate events are idempotent, and an older `subscription.active` event cannot re-enable access after a newer/equal recorded revocation.

The frontend never controls plan authority, output capacity, entitlement status, webhook verification or provider-to-plan mapping.

## Polar server configuration

The deployment supplies these values as secrets/configuration, not source code:

- `POLAR_ACCESS_TOKEN`
- `POLAR_WEBHOOK_SECRET`
- `POLAR_PRODUCT_PROFESSIONAL_ID`
- `POLAR_ENVIRONMENT` (`production` or `sandbox`)
- optional HTTPS-only `POLAR_SUCCESS_URL`
- optional HTTPS-only `POLAR_RETURN_URL`

Required Polar webhook events: `subscription.active`, `subscription.revoked`, and `order.paid`. Other signed subscription events may be retained as audit evidence but do not independently grant paid access.

No Polar public-config endpoint exists. A missing token, webhook secret, Professional product mapping, unsupported environment, invalid redirect URL, unknown provider or mismatched customer/product evidence fails closed.

## Canonical paid plan

| Plan | Canonical ID | Current output capacity |
| --- | --- | ---: |
| Professional | `professional` | 100 |

The capacity describes the current entitlement implementation. Product pages must not describe unfinished security modules as production-ready merely because the plan exists.

## Evidence and retention

Verified billing webhook payloads are not stored verbatim. The provider-neutral billing ledger stores the provider/event identity, subscription/product/customer binding fields required for reconciliation, the event time, and a SHA-256 digest of the raw signed payload. This preserves idempotency and provenance without retaining the payment payload itself.

## Historical records

Existing databases may contain Paddle schema/history from retired billing experiments. Applied migrations and historical rows remain for audit and migration integrity, but Paddle is not an accepted runtime provider and has no active checkout, webhook, public-config or browser CSP surface.

## Canonical Professional price

The public Koschei Web3 Professional price is **USD 199**. Polar remains the hosted checkout authority for the billing interval and final payment terms. A checkout redirect never grants product access; only a verified Polar webhook may activate the Professional entitlement.


## KOSC payment channel

KOSC is an alternate settlement path into the same Professional entitlement. The canonical KOSC settlement term is **30 days of Professional access for the USD 199 reference price**. Token holdings or holder tiers never grant access.

The quote stage is fail-closed and requires an authenticated customer, a previously verified Solana mainnet wallet, an independently configured official mint and treasury, an explicit access term, on-chain mint verification and an available Jupiter Price V3 observation. A quote does not activate access.

A later settlement stage must verify a finalized Solana transaction against the stored quote before Professional can be activated. The payment channel must not affect ARVIS evidence, grading or verdict authority.


## Payment hardening update — 2026-09-28

The customer payment path now preserves several additional production invariants:

- `GET /api/auth/premium-access` reports `required_plan=professional` and the payment provider attached to the authoritative entitlement row.
- The Polar success return at `/account?billing=success` does not assume checkout success means access. The account client polls the server-side entitlement briefly and only reports Professional active after the verified webhook has activated it.
- KOSC UI copy and the canonical mint identity use the `KOSC` symbol consistently.
- KOSC configuration explicitly requires `KOSCHEI_TOKEN_NETWORK=solana-mainnet`; the example environment now includes that required setting.
- A verified KOSC settlement extends from the latest finite active Professional expiry when one exists, rather than always restarting the purchased term from the settlement timestamp. Independent provider entitlements remain separate evidence-bearing rows.
- KOSC still requires a finalized, signer-bound Solana transaction that proves sufficient raw-token decrease from the verified customer wallet and sufficient raw-token increase at the configured treasury. Holdings alone never grant access.

### Deployment gate still required

Repository completion is not production billing acceptance. Production checkout remains fail-closed until the dedicated entitlement store is configured and verified and the explicit commercial checkout gate is enabled. KOSC additionally requires the canonical access term `KOSCHEI_KOSC_ACCESS_DAYS=30`. These are deployment/business configuration, not values that should be guessed or silently defaulted by application code.


## KOSC treasury readiness — 2026-09-29

A dedicated Solana treasury public address is now configured for KOSC settlement:

- treasury: `9aCTEAfFDdMk5o1ScqHZWr8gCcUDgSKUeGc264gMQbTF`
- network: `solana-mainnet`
- canonical KOSC mint: `7X9V77axASFAV8hKqqn2EfyAz4Qz3tceN8iikfukLqy1`
- commercial contract: USD 199 reference value for 30 days of Professional access.

Production KOSC quote/settlement may therefore be enabled. The backend still fails closed unless identity, verified customer wallet, Jupiter price evidence, canonical mint, configured treasury, finalized transaction evidence and entitlement storage all verify.

No seed phrase, private key or browser wallet secret belongs in Railway, GitHub, logs, support messages or the Koschei database. Only the public treasury address is deployment configuration. Token holdings alone continue to grant no access.
