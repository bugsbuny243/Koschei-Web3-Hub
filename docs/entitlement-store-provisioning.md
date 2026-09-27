# Dedicated entitlement store provisioning

Koschei Web3 keeps commercial authorization separate from application persistence.

Provision a dedicated PostgreSQL database, run:

```sql
\i scripts/provision-entitlement-store.sql
```

Then set only that database URL as `ENTITLEMENT_DATABASE_URL`.

The runtime verifies the commercial schema at startup and fails closed when the
configured database is unavailable or incomplete. The checkout redirect is not
authorization: Polar's verified webhook is the only billing event allowed to
activate or renew the Professional entitlement.

The public Professional price is USD 199. `KOSCHEI_COMMERCIAL_CHECKOUT_ENABLED`
must remain false until the dedicated ledger is connected and an end-to-end
checkout/webhook/entitlement acceptance run has passed.
