# Koschei-Web3-Hub: isolated local Docker packaging (v1)

This is a **developer-only, stateless, non-production** smoke environment.
It uses the repository's pre-existing root `Dockerfile` without changing any
Go service logic, migration, route or payment code.

## Commercial invariants

- **Polar is the sole intended commercial payment provider.**
- **Professional is the sole intended paid package.**
- This packaging change does **not** edit, migrate, enable, disable or
  reinterpret existing entitlements, packages, prices, API-key access,
  usage quotas or Polar webhooks.
- Historic other-provider references in legacy source code are **not**
  enabled, promoted or altered by these files.
- No real payment, subscription, checkout or API-key sale can be run in
  this local smoke container: it has no entitlement database or payment
  credentials. Paid operations remain fail-closed.

## Prerequisites

- Docker Engine and Docker Compose v2
- GNU Make (or a compatible Make implementation)
- Curl for the optional liveness check
- Go matching `koschei/api/go.mod` only for `make test`; Go is not
  required on the host for `make build`

## Local use

Run from the repository root:

```sh
make config
make build
make up
make smoke
make logs
make down
```

The API is bound to **127.0.0.1:8080** and nothing else. To avoid a local
port conflict without modifying the application's runtime port:

```sh
KOSCHEI_LOCAL_PORT=18080 make up
KOSCHEI_LOCAL_PORT=18080 make smoke
KOSCHEI_LOCAL_PORT=18080 make down
```

`make smoke` calls `GET /health` and verifies HTTP liveness; **it does
not assert** schema readiness, account entitlement, scan correctness,
provider capability or payment health.

## Isolation

The Compose service passes an explicit and small configuration. It forces
`APP_ENV=development`, `KOSCHEI_RUNTIME_ROLE=api`, disables background
workers and automatic scanning, and leaves
`APP_DATABASE_URL`, `ENTITLEMENT_DATABASE_URL`, `DATABASE_URL`,
Polar credentials, owner secrets and signing keys **unset**.

The API therefore uses the existing stateless mode, which cannot write a
production customer or entitlement database. There is no PostgreSQL,
Redis or NATS container, startup migration, payment simulation, bot
publisher or on-chain transaction integration in this setup.

Container hardening includes non-root execution from the existing
Dockerfile, a read-only root filesystem, an ephemeral 32 MiB `/tmp`,
dropped Linux capabilities and a no-new-privileges restriction. No
secrets or host paths are mounted. The new `.dockerignore` excludes
local env files, private-key formats and local project credentials from
the build context. Review any other new secret file patterns before
building.

## Contract verification

```sh
make config
node scripts/verify-local-packaging.cjs
make fmt-check
make test
```

The contract script invokes `docker compose config --format json`
and fails if the local service gains a privileged runtime, extra
services, non-loopback port, database or commercial secret injection,
host mounts or billing-related environment variables.

GitHub Actions `Local Packaging Smoke` builds the existing Dockerfile,
starts the ephemeral stateless API, and checks that `/health` reports
`persistence=stateless` and `database=not_used`. This is a **local
packaging acceptance**, not a production release gate.

## Production exclusions

Do **not** use this Compose file as a production deployment template.
Real deployments require independent credential injection, database
migrations, TLS/ingress, signed evidence, rate controls, production
health/readiness checks, backup and recovery, secrets management,
human-reviewed operator permissions and a production release process.

There are **no changes to the existing Polar + Professional commercial
system** in this PR.
