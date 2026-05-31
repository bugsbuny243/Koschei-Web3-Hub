# Neon Postgres setup

Run `sql/2026_05_31_koschei_web3_hub_schema.sql` against the existing Koschei Web3 Hub Neon database before starting the app. The migration is idempotent, extends the existing `plans` and `payment_requests` tables, creates the Web3 Hub persistence tables, and seeds the Starter, Builder, and Studio packs.

Set `DATABASE_URL` only in the server environment. Never prefix it with `NEXT_PUBLIC_`. The application uses Neon's server-side SQL-over-HTTP endpoint and does not send the connection string to browser code.

Shopier payment confirmation is intentionally manual for now: newly submitted payment requests remain `pending`. The unlinked owner-only `/admin` area lets the configured owner verify or reject a pending request. Verification calls `reviewPaymentRequest()`, sets the request to `manual_verified`, and activates its entitlement exactly once. A Shopier webhook can be added later if an authenticated provider flow becomes available.
