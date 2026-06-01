# Neon Postgres setup

Run `sql/2026_05_31_koschei_web3_hub_schema.sql` against the existing Koschei Web3 Hub Neon database before starting the app. The migration is idempotent and removes the legacy `password_hash` column because Neon Auth owns password verification. Public members authenticate through Neon Auth; `app_user_profiles` stores only the member profile, plan, and credit data keyed by `auth_subject`.

Set `DATABASE_URL` only in the server environment. Never prefix it with `NEXT_PUBLIC_`. The application uses Neon's server-side SQL-over-HTTP endpoint and does not send the connection string to browser code.

Shopier payment confirmation is intentionally manual for now: newly submitted payment requests remain `pending`. The unlinked admin-only `/admin` area lets the configured administrator verify or reject a pending request. Verification calls `reviewPaymentRequest()`, sets the request to `manual_verified`, and activates its entitlement exactly once. A Shopier webhook can be added later if an authenticated provider flow becomes available.
