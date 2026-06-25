# KOSCHEİ WEB3 — ARVIS

KOSCHEİ WEB3 is a live, Solana-native risk-intelligence infrastructure layer built to help developers, wallets, launchpads, research teams and investors inspect risk before interacting with a token, pool, wallet, transaction, program or claim surface.

Solana is the first live market. The architecture is designed to expose reusable security intelligence through APIs, dashboards and open technical components, while remaining extensible beyond one chain without changing the core customer experience.

## Developer Infrastructure

Koschei turns fragmented on-chain evidence into structured, integration-ready risk outputs for Solana products and research workflows.

Primary developer-facing surfaces include:

- token and liquidity risk analysis
- wallet, counterparty and sybil intelligence
- transaction decoding and execution-risk visibility
- standardized security summaries
- authenticated APIs and live verdict feeds
- public, rate-limited risk badges
- technical documentation and integration examples

The long-term open-source plan focuses on reusable ecosystem components such as transaction-decoding utilities, risk-rule helpers, example clients and integration starters. Open components must preserve the same evidence boundaries used by the live product: unavailable or unverified evidence must never be represented as a confirmed risk finding.

## Technical Outputs

Koschei is developed as technical infrastructure rather than a community or education program. Its concrete outputs are:

1. a live Solana-native risk engine
2. developer-facing API endpoints
3. structured token, wallet and transaction risk objects
4. a working dashboard and radar environment
5. reusable open-source technical components
6. documentation and integration examples for external builders

## Product Rule

```text
14 internal evidence arms
        ↓
one ARVIS core
        ↓
one customer-facing verdict card
```

The 14 arms are not sold as 14 separate products. They collect and verify evidence internally. Customers see one understandable result with the evidence, risk level and recommended next action.

## Evidence Policy

ARVIS follows a strict evidence boundary:

```text
verified evidence exists  → signed verdict may be produced
verified evidence missing → no score, no grade, no signed card
```

On-chain and off-chain evidence are labeled separately. A parsed claim URL is never represented as on-chain evidence. A program relation is never represented as confirmed sybil behavior without the required buyer and funding graph.

ARVIS does not promise guaranteed safety and does not provide investment advice. A monitor result means no critical risk evidence was found in the current evidence window; it is not a guarantee.

## Fourteen Arms

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

## Live Pipeline

```text
Raydium + Pump program activity
        ↓
transaction parsing
        ↓
project-mint resolution
        ↓
14-arm evidence analysis
        ↓
Final Verdict Engine
        ↓
one visible risk or monitor card
```

The stream processor is idempotent. The same stream event and arm cannot create duplicate verdicts. Processing jobs expose health states such as:

```text
healthy
processing
degraded
stale
waiting_for_stream
waiting_for_enriched_targets
waiting_for_processing
```

## Evidence Sources

ARVIS uses one canonical Solana RPC configuration. Resolution order:

```text
SOLANA_RPC_URL
Alchemy / Helius / QuickNode provider URL
Alchemy API key
Solana public mainnet fallback
```

The active provider is reported from the real runtime configuration rather than a hard-coded provider label.

Current evidence surfaces include:

- token mint and freeze authority
- token supply and holder concentration
- account owner and executable state
- transaction timing and failure observations
- parsed program relations
- token-balance and SOL-balance changes
- creator/signing candidates without identity claims
- initialization funding links
- Raydium interaction evidence
- Pump program interaction evidence
- priority-fee, compute-budget and route exposure
- claim URL structure and signing/secret-request indicators

## Live Product Surfaces

```text
/                         landing page
/dashboard                customer command center
/security-radar           live ARVIS radar
/reports                  signed report vault
/pricing                  plans
/health                   sanitized service and ARVIS pipeline health
/api/v1/unified/analyze   paid unified analysis
/api/v1/radar/check       paid ARVIS scan
/api/v1/radar/feed        authenticated live verdict feed
/api/v1/risk/badge        public rate-limited risk badge
```

## Runtime Controls

Important non-secret variables:

```env
PORT=10000
KOSCHEI_AUTO_RADAR_ENABLED=1
ARVIS_HEARTBEAT_SECONDS=20
ARVIS_STREAM_VERDICT_SECONDS=12
RAYDIUM_PROGRAM_ID=
PUMP_FUN_PROGRAM_ID=
```

Secrets and production credentials belong in the deployment environment and are not committed to this repository.

## Data and Infrastructure

```text
Go API
Neon Postgres
Railway production deployment
Solana RPC provider with public fallback
Vanilla HTML / CSS / JavaScript customer surfaces
```

Database migrations create the radar event store, verdict store, processing queue, recovery state and stream-verdict idempotency constraints.

## Payments and Access

Paid analysis is entitlement-backed. A profile label alone cannot unlock premium output.

```text
active entitlement + remaining output → analysis allowed
failed evidence collection             → no output charged
successful evidence-backed analysis    → one output consumed
```

Jito protected send reserves an output before submission and refunds it when submission definitively fails.

## Development

```bash
git clone https://github.com/bugsbuny243/Koschei-Web3-Hub.git
cd Koschei-Web3-Hub/koschei/api
go test ./...
go vet ./...
go build ./...
```

GitHub Actions runs tests, vet and build checks for API changes. Railway remains the production deployment source of truth.

---

Built as a live Solana-native risk-intelligence system and reusable developer infrastructure layer.