# Koschei Web3 Hub - Technical Review & Findings

**Reviewer:** Defi Mantle ([@defimantle](https://github.com/defimantle))  
**Date:** 2026-09-12  
**Commit:** [`7cf9d6f`](https://github.com/defimantle/Koschei-Web3-Hub/commit/7cf9d6f4608691712232ae8726930d4f5f0960f0) (`main`)  
**Scope:** Koschei Web3 Hub only. Koschei Lang and Koschei Sentinel are excluded per the agreed scope.  
**Method:** Read-only static analysis. No code changes were made.

---

## Table of Contents

1. [Overall Architecture & Project Structure](#1-overall-architecture--project-structure)
2. [Web3 & Blockchain Integrations](#2-web3--blockchain-integrations)
3. [Developer-Facing Tooling & APIs](#3-developer-facing-tooling--apis)
4. [Frontend/Backend Alignment & UX](#4-frontendbackend-alignment--ux)
5. [Scalability & Performance Risks](#5-scalability--performance-risks)
6. [Security & Technical Risks](#6-security--technical-risks)
7. [Missing or Incomplete Components](#7-missing-or-incomplete-components)
8. [Areas to Prioritize for Improvement](#8-areas-to-prioritize-for-improvement)
9. [Recommended First Implementation Milestone](#recommended-first-implementation-milestone)

---

## 1. Overall Architecture & Project Structure

### Summary

Koschei Web3 Hub is a Go monolith deployed as a single Docker container built from `scratch` with pinned Alpine and Go digests. It is a Web3 Security Validation & Risk Intelligence platform centered around the ARVIS engine (Analysis, Risk, Verification, Intelligence, Scoring).

### Structure at a Glance

| Layer | Location | Description |
|---|---|---|
| Entrypoint | [`koschei/api/main.go`](koschei/api/main.go) | Wires DB, cache, RPC, jobs, HTTP server |
| HTTP / Routing | [`koschei/api/internal/http/`](koschei/api/internal/http) | 55 files. Server, routes, CORS, CSP, rate limiting, security headers |
| Handlers | [`koschei/api/internal/handlers/`](koschei/api/internal/handlers) | 489 files. The largest package, all API logic lives here |
| Services / Intelligence | [`koschei/api/internal/services/`](koschei/api/internal/services) | 331 files. ARVIS core, radar, evidence, funding clusters, behavioral signatures |
| Web3 / RPC | [`koschei/api/internal/web3/`](koschei/api/internal/web3) | 24 files. Solana RPC client, governor, failover, evidence court |
| Defense Engine | [`koschei/api/internal/defense/`](koschei/api/internal/defense) | 45 files. Anvil, LiteSVM harnesses, execution proofs, reproduction |
| Database | [`koschei/api/internal/db/`](koschei/api/internal/db) | Connection management, pooling, migration runner |
| Migrations | [`koschei/api/migrations/`](koschei/api/migrations) | 118 SQL migration files for Neon Postgres |
| ClickHouse | [`koschei/api/clickhouse/`](koschei/api/clickhouse) | Shadow journal for analytics. Read-only v1, not production-active |
| Frontend | [`koschei/api/public/`](koschei/api/public) | 58+ static HTML/CSS/JS files served by the Go binary |
| OSS | [`oss/`](oss) | TypeScript event normalizer, verifier, and JSON schemas |
| Fabric | [`fabric/`](fabric) | Federated workspace config, cross-project contracts |
| Commands | [`koschei/api/cmd/`](koschei/api/cmd) | 12 standalone CLI tools (ClickHouse, defense worker, publisher, etc.) |
| CI/CD | [`.github/workflows/`](.github/workflows) | 71 GitHub Actions workflows |

### Quantitative Profile

| Metric | Count |
|---|---|
| Total files (non-git) | 1,646 |
| Go source files (non-test) | 631 |
| Go test files | 526 |
| SQL migrations | 118 |
| GitHub Actions workflows | 71 |
| HTML pages | 58+ |
| Verify/acceptance scripts | ~50 |
| OpenAPI spec size | 504 KB / 16,337 lines |
| Go dependencies | 3 (`lib/pq`, `x/crypto`, `x/sys`) |

### Architectural Observations

**Lean dependency footprint.** Only 3 Go modules for a project this size. This is intentional and it eliminates most supply-chain risk.

**Monolithic handler package.** `internal/handlers/` has 489 files in a flat directory. Go compiles this fine but navigating it as a developer is rough. Feature-grouped sub-packages would help.

**Five-plane security model.** The codebase follows a clear model: Evidence, Execution Integrity, Signing Defense, Infrastructure Defense, Decision. Documented in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/SECURITY_PRODUCT_CHARTER.md`](docs/SECURITY_PRODUCT_CHARTER.md). Consistently implemented.

---

## 2. Web3 & Blockchain Integrations

### Solana (Production Core)

The Solana integration is the real deal. Deep and clearly production-tested:

- RPC client with primary/fallback resolution, rate limiting, budget tracking, batch mode, failover, 429-backoff. See [`koschei/api/internal/web3/solana_rpc.go`](koschei/api/internal/web3/solana_rpc.go)
- Provider abstraction supporting Alchemy, Helius, QuickNode, and Solscan with cost-tier gating
- Evidence court with canonical evidence evaluation. See [`koschei/api/internal/web3/evidence_court.go`](koschei/api/internal/web3/evidence_court.go)
- PumpPortal WebSocket integration for real-time Pump.fun token discovery
- Raydium CLMM and AMM LP evidence collection
- Jupiter trusted market context and swap quote adapters
- Token-2022 extension scanning
- Transaction Guard v2/v3 with CPI flow analysis, authority binding, signed enforcement permits
- Defense OS with Anvil and LiteSVM execution harnesses for smart contract reproduction

### EVM Chains (Probe-Ready)

- Ethereum, Base, Arbitrum, and Optimism have probe-ready adapters. See [`koschei/api/internal/http/network_evm_probe_routes.go`](koschei/api/internal/http/network_evm_probe_routes.go)
- These are address/transaction resolution probes, not full collectors
- No EVM-specific evidence court or verdict engine yet

### Bitcoin (Probe-Ready)

- Bitcoin mainnet address parsing and probe support was recently added
- UTXO model awareness is present in the network target routing

### Chain Adapter Boundary

The architecture correctly treats chain adapters as evidence transports, not product identities. Core intelligence (ARVIS verdict engine, attack-path reasoning, behavioral signatures) is chain-independent. Good foundation for multi-chain expansion.

EVM and Bitcoin probes exist but lack the depth of Solana. No EVM evidence court, no EVM transaction guard, no EVM radar pipeline. These are correctly labeled `probe_ready` in the codebase and not falsely claimed as production capability.

---

## 3. Developer-Facing Tooling & APIs

### API Surface

The API is organized into 9 route groups. See [`koschei/api/internal/http/route_inventory.go`](koschei/api/internal/http/route_inventory.go):

1. **Public & System** - health, config, impact metrics, public risk badges
2. **Identity** - Neon Auth (OIDC/JWKS), local auth, register/login
3. **Account** - API key management (Professional tier)
4. **Billing** - Polar hosted SaaS checkout/webhooks
5. **Owner** - full operator command center (radar, defense, chat, intelligence)
6. **Professional Customer** - token scan, transaction preflight, radar, court
7. **Developer API** - API-key-authenticated B2B scan, shield, defense validation
8. **Dossier** - public case registry and dossier exports
9. **Watchlist & Webhooks** - alerting pipeline with Telegram/Discord delivery

### OpenAPI Specification

- 504 KB OpenAPI spec at [`koschei/api/openapi.yaml`](koschei/api/openapi.yaml), 16,337 lines
- Auto-generated and contract-tested in CI

### OSS Packages

- [`oss/event-normalizer/`](oss/event-normalizer) - TypeScript library for normalizing Pump/Raydium events
- [`oss/verifier/`](oss/verifier) - dossier verification tools
- [`oss/schemas/`](oss/schemas) - signed verdict JSON schema

### Developer Documentation

- [`docs/DEVELOPER_QUICKSTART.md`](docs/DEVELOPER_QUICKSTART.md) - basic but functional
- [`docs/API_REFERENCE.md`](docs/API_REFERENCE.md) and [`docs/api-reference.md`](docs/api-reference.md) - two separate reference docs exist

Developer onboarding docs are thin for a project this complex. The quickstart mentions `go test ./...` but doesn't explain how to run the full stack locally, set up a test Postgres instance, or simulate the radar pipeline. No SDK examples to speak of.

---

## 4. Frontend/Backend Alignment & UX

### Current Frontend

The frontend is static HTML pages served directly by the Go binary's file server:

- Landing page ([`koschei/api/public/index.html`](koschei/api/public/index.html)) - clean, evidence-first messaging
- Customer dashboard, scan page, security radar, cases, watchlist
- Owner panel with admin operations
- Auth flow (login, register, password reset)
- Legal pages (terms, privacy, refund policy)

### Architecture

Frontend communicates with the backend through `fetch()` calls to `/api/*` endpoints. No build step, no bundler. Pure HTML/CSS/JS.

### CSS

A dedicated CSS file ([`koschei/api/public/css/koschei-home.css`](koschei/api/public/css/koschei-home.css)) provides the styling. Dark-themed, functional, fits the security product identity.

### Observations

The static HTML approach lines up with the architecture's trust model: the frontend is explicitly untrusted, all security decisions happen server-side. Correct design for a security product.

However, there is no component framework. Headers, nav, and footers are copy-pasted across 58+ files. Even a basic Go `html/template` include pattern would cut down on this duplication.

CSP is dynamically generated per-response by [`koschei/api/internal/http/csp_html.go`](koschei/api/internal/http/csp_html.go) with nonce-based script isolation. Well done.

---

## 5. Scalability & Performance Risks

### Database Layer

- Neon Postgres is the primary store with configurable connection pooling (default: 5 max open, 0 idle)
- Read replica support via `APP_DATABASE_READ_URL`
- Entitlement ledger gets its own database handle, good isolation
- ClickHouse shadow journal in place for future analytics offloading

### Caching

- Three-tier cache: Redis, then in-memory, then noop. Degrades gracefully.
- RPC responses, evidence, and intelligence results are cached

### RPC Budget Management

- Solana RPC budget system with request counting, 429 handling, cooldowns, per-cycle limits
- Singleflight deduplication for concurrent identical requests
- Batch RPC support for transaction fetching

### Job Queue

- NATS-based job queue for async investigation jobs
- Worker wake-up is event-driven with a bounded fallback ceiling (avoids high-frequency polling)

### Risks

**Sequential startup migrations.** 118 SQL migrations run one after another on boot. Cold-start latency is a concern for larger deployments, and there is potential for lock contention. Migration numbering has gaps (026-027, 048-052, 061, 082, 085, 102) from historical cleanup, but the system handles it.

**Single-process architecture.** The main API, radar workers, PumpPortal WebSocket listeners, ARVIS stream processors, and alert delivery all run in one Go process. A CPU-heavy evidence collection task could starve HTTP request handling. The defense worker has its own [`Dockerfile.defense-worker`](Dockerfile.defense-worker), but the core API is still a monolith.

**RPC budget system is solid.** Rate limiting, backoff, cooldown, and per-cycle caps show real-world experience operating against provider quotas. No complaints here.

---

## 6. Security & Technical Risks

### Strengths

1. **Minimal dependency surface** - 3 Go modules total. Eliminates most supply-chain attack vectors.
2. **Docker from scratch** - no shell, no OS utilities in the production container. Runs as non-root user `10001:10001`. See [`Dockerfile`](Dockerfile).
3. **Pinned base images** - Go and Alpine images both use SHA-256 digest pinning
4. **Security headers** - X-Frame-Options, CSP with nonces, HSTS, COOP, CORP, Permissions-Policy. See [`koschei/api/internal/http/security_headers.go`](koschei/api/internal/http/security_headers.go)
5. **Sensitive path blocking** - `.env`, `.git`, `.ssh`, private keys, Docker configs all return 404 from the static file server
6. **No secrets in code** - [`.env.example`](.env.example) is templates only. [`.gitleaksignore`](.gitleaksignore) is present.
7. **Fail-closed design** - missing evidence stays `UNKNOWN`, missing entitlements fail-closed, missing database is explicit
8. **Auth hardening** - JWKS cache hardening, redirect validation rejects backslash/CR/LF
9. **71 CI workflows** including CodeQL, supply-chain security, auth freeze guard, security CI

### Risks

**Production persistence is broken right now.** Per [`docs/PRODUCTION_BLOCKER_2026-09-12.md`](docs/PRODUCTION_BLOCKER_2026-09-12.md), enabling `APP_DATABASE_URL` caused a credential auth failure (`pq: password authentication failed`). The system is running stateless, so database-backed features (watchlists, job history, owner operations, public case registry) are all unavailable in production. This is the most critical issue.

**`ADMIN_PASSWORD` is plaintext in environment.** Owner login does a direct password comparison against an env var. Behind HTTPS and rate-limited, but still a legacy pattern that should be hashed.

**Migration numbering conflicts.** Several numbers are duplicated (029, 033, 034, 040, 041, 042, 043, 078, 081, 091). The runner uses filename-based idempotency so both files in each pair execute, but it creates confusion and potential ordering problems.

---

## 7. Missing or Incomplete Components

### Confirmed Missing

| Component | Status | Impact |
|---|---|---|
| Production database connectivity | Blocked by credential issue | Watchlists, job history, public cases unavailable |
| EVM evidence court | Not started | No deep EVM security analysis |
| EVM transaction guard | Not started | No EVM transaction preflight |
| Bitcoin evidence collection | Probe only | Address resolution only |
| SDK / client libraries | Minimal | No official Go/JS/Python SDKs |
| Developer documentation | Incomplete | Only a basic quickstart exists |
| Frontend component system | Absent | 58+ duplicated HTML files |
| Structured logging | Absent | All logging is `log.Printf` to stdout |
| Metrics / Prometheus | Absent | No exported metrics for monitoring |
| Rate limiting persistence | In-memory only | Resets on restart, not shared across instances |
| API versioning strategy | Informal | Mix of `/api/`, `/api/v1/`, unversioned paths |

### Incomplete / In-Progress

| Component | Status | Reference |
|---|---|---|
| CORE-01: Entitlement split-plane | Implementation in progress | [`PROJECT_STATE.md`](PROJECT_STATE.md) |
| CORE-02: Readiness/CI truthfulness | In progress | [`PROJECT_STATE.md`](PROJECT_STATE.md) |
| CORE-04: Cross-project acceptance | Schema exists, tests not executed | [`PROJECT_STATE.md`](PROJECT_STATE.md) |
| SIGN-01: Signing defense acceptance | Open | [`PROJECT_STATE.md`](PROJECT_STATE.md) |
| ClickHouse live path | Shadow-only, read/verify | [`koschei/api/clickhouse/README.md`](koschei/api/clickhouse/README.md) |
| Fabric integration | Experimental, observe-mode | [`fabric/`](fabric) |

---

## 8. Areas to Prioritize for Improvement

### Priority 1 - Critical (Blocking Production)

1. **Resolve the production database credential blocker.** Without application persistence, the public case registry, watchlists, job history, and owner operations are all down. This blocks most of the product's real capabilities.

2. **Complete the CORE-01 entitlement split-plane.** Separating entitlement/billing from application persistence is the right call architecturally, but it is incomplete. Paid customer access fails closed without a configured entitlement store.

### Priority 2 - High (Developer Productivity & Reliability)

3. **Add structured logging.** Replace `log.Printf` with `slog` from the Go standard library. Necessary for production debugging, alerting, and log aggregation.

4. **Add Prometheus/OpenTelemetry metrics.** No observable metrics exist today. Need to export RPC budget usage, cache hit rates, scan latencies, error rates.

5. **Fix migration numbering conflicts.** Deduplicate the 10 conflicting migration numbers to prevent ordering issues going forward.

### Priority 3 - Medium (Product Quality)

6. **Improve developer documentation.** Write a real local development guide, API usage examples, and SDK starters. The 504 KB OpenAPI spec is not enough on its own.

7. **Introduce frontend templating.** A Go `html/template` include pattern for shared headers/footers would cut the duplication across 58+ HTML files.

8. **Formalize API versioning.** Define which routes are stable (`/api/v1/`) and which are internal or experimental.

### Priority 4 - Future (Strategic)

9. **EVM evidence depth.** Move Ethereum/Base/Arbitrum/Optimism from probe-ready to real evidence collection when the Solana core is fully stable.

10. **Horizontal scaling.** Move rate-limit state and session state to Redis or the database so the API can run multiple instances behind a load balancer.

---

## Recommended First Implementation Milestone

### Milestone: Production Persistence Recovery & Observability Foundation

**Goal:** Fix the production database blocker and put a basic observability stack in place so the platform can run its full feature set and be monitored.

**Deliverables:**

1. **Database connectivity audit.** Verify the `APP_DATABASE_URL` credential against the Neon endpoint, fix the connection string, confirm startup succeeds with persistence enabled.

2. **Structured logging migration.** Replace `log.Printf` calls with Go `slog` structured logging in `main.go`, `db.go`, and the HTTP server layer. Mechanical change, low risk, immediate improvement to production diagnostics.

3. **Health endpoint enrichment.** Extend `/health` to report persistence status, entitlement store status, cache status, and RPC status in a machine-readable format. Enables monitoring integration.

4. **Migration hygiene.** Resolve the 10 duplicate migration numbers into a clean sequence. Add a migration README documenting the numbering contract.

5. **End-to-end smoke test.** With persistence enabled, verify: `/api/public/cases` returns `200`, owner login works, a token scan completes, watchlist is accessible.

**Why this milestone first:**

- Unblocks all database-backed features without touching product logic
- Establishes the observability foundation every later milestone depends on
- Entirely additive, nothing existing gets removed or broken
- Produces concrete evidence (health output, passing smoke tests) that can be shared with stakeholders

**Estimated scope:** 1 Week.
