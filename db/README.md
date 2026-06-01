# Neon Postgres setup

Run `sql/2026_05_31_koschei_web3_hub_schema.sql` and then `sql/2026_06_01_member_accounts.sql` against the existing Koschei Web3 Hub Neon database before starting the app. Both migrations are idempotent. The member-auth migration creates the isolated `member_accounts` table without deleting or reusing `app_user_profiles`.

Set `DATABASE_URL` only in the server environment. Never prefix it with `NEXT_PUBLIC_`. The application uses Neon's server-side SQL-over-HTTP endpoint and does not send the connection string to browser code.

Shopier payment confirmation is intentionally manual for now: newly submitted payment requests remain `pending`. The unlinked owner-only `/admin` area lets the configured owner verify or reject a pending request. Verification calls `reviewPaymentRequest()`, sets the request to `manual_verified`, and activates its entitlement exactly once. A Shopier webhook can be added later if an authenticated provider flow becomes available.
