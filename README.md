# KOSCHEİ WEB3 — ARVIS

Koschei ARVIS is a live, Solana-native pre-signing risk layer for developers, wallets, launchpads, dApps, DeFi protocols, research teams and security operators.

## 30-second pitch

Koschei ARVIS stops risky Solana interactions before users sign. Integrating products call one API and receive a machine-readable **allow, warn, block or withhold** decision backed by verified evidence, rule metadata and signed status.

Instead of building separate token, wallet, transaction, monitoring and alert systems, Solana product teams integrate one reusable risk layer.

## Who pays — and why

| Customer | What ARVIS does | Why they pay |
| --- | --- | --- |
| Wallets | Adds evidence-backed pre-signing warnings | Reduce preventable risky interactions and support incidents |
| Launchpads | Screens token behavior, authorities and concentration | Avoid maintaining several disconnected screening systems |
| dApps and DeFi protocols | Simulates transactions and applies integration policy | Make consistent allow, warn, block or withhold decisions before execution |
| Security and research teams | Monitors targets and delivers signed alerts | Replace manual monitoring with auditable evidence and reliable delivery |

Commercial access can be output-based API capacity, persistent monitoring capacity or a B2B integration agreement.

## Why Solana

ARVIS is not a generic Web3 score with a Solana label. Its evidence model is built around:

- Solana transaction instructions and account relationships
- SPL Token and Token-2022 authorities, extensions and transfer behavior
- mint and freeze authority evidence
- program-specific relations and liquidity activity
- priority-fee and pre-signing simulation context
- Pump-style launch observations
- Raydium-oriented liquidity evidence

## Current proof and next proof

Live today:

- production Go API and worker pipeline
- evidence-backed radar and signed verdict contract
- Token-2022 scanner and transaction firewall
- persistent watchlists and HMAC-signed webhook delivery
- authenticated B2B batch screening with idempotency
- asynchronous result lookup and usage accounting
- TypeScript client, schemas, examples and CI checks

The next proof is external adoption: integration pilots, measured reliability and published technical case studies. Pilot requests are collected at `/pilot`.

## Technical scope

Koschei is an engineering and infrastructure project. Its concrete outputs are:

1. a live Solana observation and risk-processing pipeline
2. deterministic evidence collection and final verdict generation
3. authenticated developer APIs
4. a TypeScript SDK
5. an open-source Solana event normalizer
6. a machine-readable signed-verdict schema
7. wallet and launchpad integration examples
8. developer documentation and reproducible CI checks

Community events, general education and ecosystem promotion are not the product scope.

## Open-source developer kit

| Component | Location | Status |
| --- | --- | --- |
| TypeScript API client | `sdk/typescript` | Shipped and tested |
| Solana event normalizer | `oss/event-normalizer` | Shipped and tested |
| Signed verdict schema | `oss/schemas/signed-verdict.schema.json` | Shipped |
| Wallet warning example | `examples/wallet-warning` | Shipped |
| Launchpad screening example | `examples/launchpad-screening` | Shipped |
| API reference | `docs/api-reference.md` | Shipped |
| Developer quickstart | `docs/DEVELOPER_QUICKSTART.md` | Shipped |
| Grant evidence matrix | `docs/grant-evidence-matrix.md` | Shipped |
| Pitch one-pager | `docs/pitch-one-pager.md` | Shipped |

The open-source packages are MIT licensed and designed to remain useful without the hosted dashboard.

## Product rule

```text
14 internal evidence arms
        ↓
one ARVIS core
        ↓
one customer-facing verdict
```

The evidence arms are internal verification layers, not separate products. Customers and integrations receive one structured output with evidence, risk level, rule version and recommended action.

## Evidence policy

```text
verified evidence exists  → signed verdict may be produced
verified evidence missing → no score, no grade, no signed verdict
```

On-chain and off-chain observations are labeled separately. Parsed URLs are not presented as on-chain evidence. Wallet relations are not presented as real-world identity claims. A low-risk or monitor result is not a safety guarantee.

## Core evidence arms

1. Pump.fun Sybil Radar
2. Raydium Pool Guardian
3. Walletless Claim Shield
4. Intelligence Graph
5. MEV Shield
6. Token Authority Scanner
7. Holder Concentration
8. Liquidity Movement
9. Creator Link Analysis
10. Funding Cluster Detector
11. Sniper Timing Detector
12. Claim Surface Risk
13. Program Relation Scan
14. Final Verdict Engine

Each arm remains unsigned when its required evidence is unavailable.

## Live pipeline

```text
Pump-style + Raydium-style observations
        ↓
transaction and account enrichment
        ↓
target normalization
        ↓
evidence-arm processing
        ↓
Final Verdict Engine
        ↓
signed risk, monitor or withheld output
```

The stream processor is idempotent. The same stream event and evidence arm cannot create duplicate final output. Processing states include healthy, processing, degraded, stale, retryable failure and exhausted failure.

## Provider resilience

ARVIS resolves one canonical Solana RPC provider at startup and applies process-wide pacing and retry controls. When the configured production provider is rate-limited or unavailable, standard Solana RPC calls automatically fall back to the public Solana mainnet endpoint.

Provider-specific limits never authorize the system to fabricate evidence. Unsupported or unavailable evidence results in a withheld or partial analysis.

## Developer routes

```text
POST /api/v1/radar/check        session-authenticated radar check
GET  /api/v1/radar/feed         authenticated verdict feed
POST /api/v1/scan/token         API-key token and batch scan
POST /api/v1/shield/preflight   API-key pre-signing risk check
POST /api/v1/shield/transaction API-key transaction simulation
GET  /api/v1/usage              API-key usage and async results
GET  /api/v1/risk/badge         public rate-limited risk badge
```

See `docs/api-reference.md` for authentication boundaries and current production status.

## Integration pilot

The pilot flow is for wallets, dApps, launchpads, DeFi protocols and security teams with a real Solana integration surface.

A strong pilot has:

- one named integration owner
- one explicit risk decision
- one documented live API route
- measurable decision latency and completed-check rate
- reviewed false-positive and withheld-output samples
- permission to publish anonymized technical integration notes

Apply through the production `/pilot` page.

## Local validation

### Go API

```bash
git clone https://github.com/bugsbuny243/Koschei-Web3-Hub.git
cd Koschei-Web3-Hub/koschei/api
go test ./...
go vet ./...
go build ./...
```

### TypeScript SDK

```bash
cd sdk/typescript
npm install
npm run check
npm test
npm pack --dry-run
```

### Event normalizer

```bash
cd oss/event-normalizer
npm install
npm run check
npm test
npm pack --dry-run
```

## Production architecture

```text
Go API and workers
Railway deployment
Neon PostgreSQL
Solana RPC provider with automatic public fallback
Pump-style stream observations
Vanilla HTML / CSS / JavaScript customer surfaces
```

Secrets and production credentials live only in the deployment environment and are not committed to the repository.

## Access model

Paid analysis is entitlement-backed. A profile label alone cannot unlock premium output.

```text
active entitlement + remaining output → analysis allowed
failed evidence collection             → no output charged
successful evidence-backed analysis    → one output consumed
```

## Documentation

- Architecture: `docs/ARCHITECTURE.md`
- Data flow: `docs/architecture/data-flow.md`
- API reference: `docs/api-reference.md`
- Developer quickstart: `docs/DEVELOPER_QUICKSTART.md`
- Signed verdict contract: `docs/signed-verdict-schema.md`
- Limitations: `docs/limitations.md`
- Technical whitepaper: `docs/technical-whitepaper.md`
- Open-source roadmap: `docs/open-source-roadmap.md`
- Pitch one-pager: `docs/pitch-one-pager.md`
- Grant resubmission: `docs/grant-v3-proposal.md`
- Grant evidence matrix: `docs/grant-evidence-matrix.md`

## License

MIT — see `LICENSE`.

---

Built as live Solana-native pre-signing risk infrastructure and reusable developer tooling.