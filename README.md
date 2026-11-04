# Koschei Web3 Hub

Evidence-oriented Web3 security platform with ARVIS verdicts, transaction guard and state recheck, multi-chain radar, Tron Flow Radar, SOC workflows, and Active Response shadow planning.

## Architecture and runtime

The Go API lives in `koschei/api`. `KOSCHEI_RUNTIME_ROLE` selects `combined`, `api`, or `worker`. `APP_DATABASE_URL` enables application persistence and `APP_DATABASE_READ_URL` optionally configures a read replica. `DATABASE_URL` is used by legacy services and tools; `ENTITLEMENT_DATABASE_URL` is the separate commercial authorization store. These database roles are not interchangeable.

Tron Flow Radar produces on-chain observations. Evidence packages use an in-memory finding or a persisted `security_alert_events` row when application persistence is configured. Signing requires a configured platform signing key; absent signing is explicitly reported, not fabricated.

Active Response is a shadow-planning boundary, not a production execution engine. Do not interpret a proposed action as a completed defensive action.

## Development

Read [AGENTS.md](AGENTS.md) before changing behavior. Review the root Makefile for `make up`, `make smoke`, and `make test` targets. In `koschei/api`, run `go test ./...` and `go vet ./...` using the Go toolchain specified in `go.mod`.

## Documentation

- [Documentation index](docs/INDEX.md)
- [Railway environment guide](docs/RAILWAY_ENV_REHBERI.md)
- [Chain capability matrix](docs/CHAIN_CAPABILITY_MATRIX.md)
- [Product charter](docs/PRODUCT_CHARTER_WEB3_V1.md)
- [Contributor instructions](AGENTS.md)
