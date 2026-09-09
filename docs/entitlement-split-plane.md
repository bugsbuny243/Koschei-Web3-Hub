# Koschei Web3 entitlement split-plane

Status: integration candidate; not production acceptance evidence.

Koschei Web3 keeps application investigation persistence and commercial authorization as separate trust/availability planes.

## Application plane

`Handler.DB` remains the application persistence handle. In the stateless Web3 runtime it is intentionally `nil`.

When it is nil, durable jobs, watchlists, historical investigation readback, database-backed alerting and other stateful application routes stay fail-closed. The runtime does not implicitly read `DATABASE_URL` to recover them.

## Entitlement plane

`Handler.EntitlementDB` is a narrowly scoped commercial ledger used for:

- binding the authenticated subject/email to an active customer profile;
- validating the canonical Professional entitlement;
- checking remaining paid outputs;
- atomically reserving/refunding an output;
- Polar checkout customer binding;
- Polar signed-webhook event deduplication and entitlement activation/renewal/revocation.

The stateless runtime reads only `ENTITLEMENT_DATABASE_URL` for this plane. It never treats frontend checkout state, payment-provider metadata alone, a token balance, or a missing ledger as authorization.

`ConnectEntitlementStore` does not run application migrations. It verifies the expected commercial ledger shape and fails startup when an explicitly configured store is unavailable or incompatible.

If `ENTITLEMENT_DATABASE_URL` is absent, public/free stateless surfaces may continue to run but paid customer operations remain fail-closed.

Existing stateful deployments remain compatible: when no dedicated `EntitlementDB` is injected, entitlement lookups may fall back to `Handler.DB`.

## Route boundary

The entitlement-aware HTTP readiness layer treats three classes separately:

1. application-database routes — require `Handler.DB`;
2. entitlement-only billing routes (`/api/polar/checkout`, `/api/polar/webhook`) — bypass application DB readiness but require the entitlement store;
3. stateless Professional operations — `/api/v1/radar/check` uses its route-level Professional/output gate, while legacy `/api/arvis/preflight` and `/api/token/scan` are wrapped by the entitlement-aware compatibility gate.

A route being application-DB-optional is never an authorization bypass.

## CORE-01 acceptance still required

This design removes the specific nil-application-DB versus entitlement-gate contradiction, but CORE-01 remains open until the exact candidate is exercised against an authorized test/customer ledger and the full checkout/webhook → entitlement → analysis → quota/refund lifecycle is verified. Durable scan/report/readback requirements must be evaluated separately against the deliberately disabled application persistence plane rather than silently claimed as complete.
