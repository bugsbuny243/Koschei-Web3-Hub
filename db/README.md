# Neon Postgres setup

Run `sql/2026_05_31_koschei_web3_hub_schema.sql` against the existing Koschei Web3 Hub Neon database before starting the app. The migration is idempotent and does not remove legacy columns. Public members authenticate through the Go `services/auth-api` service backed by Neon Auth; `app_user_profiles` stores only the member profile, plan, and credit data keyed by `auth_subject`.

Set `DATABASE_URL` only in server environments, including the Go `auth-api` service and existing server-side web data routes. Never prefix it with `NEXT_PUBLIC_`. The application uses Neon's server-side SQL-over-HTTP endpoint and does not send the connection string to browser code.

Shopier payment confirmation is intentionally manual for now: newly submitted payment requests remain `pending`. The unlinked admin-only `/admin` area lets the configured administrator verify or reject a pending request. Verification calls `reviewPaymentRequest()`, sets the request to `manual_verified`, and activates its entitlement exactly once. A Shopier webhook can be added later if an authenticated provider flow becomes available.
