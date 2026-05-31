# Neon Postgres setup

Run `db/migrations/0001_koschei_web3_hub.sql` against the Neon database before starting the app. The migration is idempotent and seeds the Starter, Builder, and Studio packs.

Set `DATABASE_URL` only in the server environment. Never prefix it with `NEXT_PUBLIC_`. The application uses Neon's server-side SQL-over-HTTP endpoint and does not send the connection string to browser code.

Shopier payment confirmation is intentionally manual for now: newly submitted manual orders remain `pending`. An administrator must verify payment and set an order to `manual_verified` or `paid` before `createEntitlementFromPaidOrder()` can activate output rights. A Shopier webhook can be added later if an authenticated provider flow becomes available.
