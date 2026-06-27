# KOSCHEİ WEB3 — ARVIS

KOSCHEİ WEB3 is a live, Solana-native risk-intelligence infrastructure layer for developers, wallets, launchpads, research teams and security operators.

ARVIS converts launch, liquidity, token, wallet, transaction, program and claim observations into evidence-backed, versioned risk outputs. Solana is the first live market.

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
POST /api/v1/radar/check       session-authenticated radar check
GET  /api/v1/radar/feed        authenticated verdict feed
POST /api/v1/scan/token        API-key token scan
POST /api/v1/shield/preflight  API-key pre-signing risk check
GET  /api/v1/usage             API-key usage records
GET  /api/v1/risk/badge        public rate-limited risk badge
```

See `docs/api-reference.md` for authentication boundaries and current production status.

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
Render deployment
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
- Grant resubmission: `docs/grant-v2-proposal.md`
- Grant evidence matrix: `docs/grant-evidence-matrix.md`

## License

MIT — see `LICENSE`.

---

Built as live Solana-native risk infrastructure and reusable developer tooling.
