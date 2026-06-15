# Koschei Web3 Hub

> **Alchemy-first automatic Solana security radar.** Koschei watches Solana activity, detects early risk patterns, signs deterministic verdicts, and gives customers a clear A-F risk grade before they interact.

![Production](https://img.shields.io/badge/Production-Online-00ffaa?style=for-the-badge)
![Solana](https://img.shields.io/badge/Solana-Mainnet-24eaff?style=for-the-badge)
![Provider](https://img.shields.io/badge/Data-Alchemy%20HTTPS-7c5cff?style=for-the-badge)
![Build](https://img.shields.io/badge/Build-Go%20API%20%2B%20Vanilla%20JS-success?style=for-the-badge)

---

## Product Direction

Koschei is not a generic chatbot and it is not a manual-only scanner. The product direction is:

```text
Koschei watches Solana.
Koschei detects risk.
Koschei signs verdicts.
Customers consume the intelligence through dashboard, API, widget and badge.
```

The first production security surface is focused on three core radars:

1. **Pump.fun Sybil Radar**
2. **Raydium Pool Guardian**
3. **Walletless Claim Shield**

The model layer is private. Customers cannot change prompts, verdict thresholds, rule weights or scoring behavior. AI can explain findings, but the final grade belongs to the deterministic Koschei rule engine.

---

## Core Radars

### 1. Pump.fun Sybil Radar

Detects coordinated launch behavior around new Pump.fun-style token launches.

Koschei tracks and scores:

```text
new token launch activity
creator wallet
first 10 / 25 / 50 / 100 buyers
shared funding-source clusters
creator-linked buyer relations
early holder concentration
sniper-like timing patterns
```

Customer output:

```json
{
  "module": "pump_sybil_radar",
  "grade": "F",
  "risk_index": 91,
  "risk_level": "critical",
  "verdict": "Coordinated launch behavior suspected",
  "recommendation": "avoid"
}
```

### 2. Raydium Pool Guardian

Detects risky Raydium pool behavior, unsafe authority state and liquidity concentration.

Koschei tracks and scores:

```text
new Raydium pools
liquidity added events
token mint account
mint authority
freeze authority
pool creator
LP concentration
top holder concentration
liquidity movement signals
```

Customer output:

```json
{
  "module": "raydium_pool_guardian",
  "grade": "D",
  "risk_index": 74,
  "risk_level": "high",
  "verdict": "High risk pool or unsafe authority state",
  "recommendation": "manual_review"
}
```

### 3. Walletless Claim Shield

Lets users check claim pages, program IDs and suspicious Solana targets before connecting a wallet.

Koschei tracks and scores:

```text
claim URLs
claim program IDs
claim token accounts
known-risk relations
unsafe transaction patterns
pre-connect warning signals
```

Customer output:

```json
{
  "module": "walletless_claim_shield",
  "grade": "F",
  "risk_index": 92,
  "risk_level": "critical",
  "verdict": "Do not connect wallet before review",
  "recommendation": "avoid"
}
```

---

## Live Product Surfaces

```text
/                       Landing page
/dashboard              Paid customer dashboard
/security-radar         Focused 3-radar command panel
/security-ecosystem     Public ecosystem/module overview
/security-ecosystem.json Machine-readable radar manifest
/widget.js              Embeddable Koschei risk badge
/owner-production.html  Owner operations panel
/api/v1/unified/analyze Paid unified analyzer
```

Widget example:

```html
<script src="https://tradepigloball.co/widget.js" data-token="TOKEN_MINT"></script>
```

---

## Data Provider Strategy

Current production direction is **Alchemy-first**.

```text
Solana HTTPS RPC  -> required
Solana WebSocket  -> optional, not required
Helius            -> optional future data acceleration, not required
Other chains      -> disabled for this Solana-first radar surface
```

The automatic radar can run with polling over Solana HTTPS RPC.

```text
SOLANA_RPC_URL
↓
Polling watcher
↓
Pump.fun / Raydium / claim activity detection
↓
Koschei deterministic rule engine
↓
Signed customer verdict
```

---

## Required Environment Variables

Minimum production variables:

```env
PORT=8080
DATABASE_URL=postgres://...
SOLANA_RPC_URL=https://solana-mainnet.g.alchemy.com/v2/...
ALCHEMY_API_KEY=...
WEB3_PROVIDER=alchemy
TOGETHER_API_KEY=...
TOGETHER_AI_ENABLED=1
TOGETHER_MODEL=Qwen/Qwen3-235B-A22B-Instruct-2507-tput
```

Security radar flags:

```env
KOSCHEI_SECURITY_MODULES=pump_sybil,raydium_guardian,claim_shield
KOSCHEI_SECURITY_PROVIDER=alchemy
KOSCHEI_AUTO_RADAR_ENABLED=1
KOSCHEI_SOLANA_WATCH_MODE=polling
KOSCHEI_RADAR_POLL_SECONDS=10
KOSCHEI_MODEL_ROUTER_ENABLED=1
KOSCHEI_VERDICT_MODE=deterministic_signed
KOSCHEI_PUBLIC_BADGE_ENABLED=1
```

Owner/auth/payment variables are configured separately in production hosting and are not documented with secret values in this repository.

---

## Architecture

```text
Customer / Partner Surface
  ├─ /security-radar
  ├─ /dashboard
  ├─ /widget.js
  └─ API consumers

Go API
  ├─ auth + entitlement checks
  ├─ unified analyzer
  ├─ deterministic security rules
  ├─ model explanation layer
  └─ owner-only operations

Data Layer
  ├─ Alchemy Solana HTTPS RPC
  ├─ Neon Postgres
  └─ optional cache

Verdict Layer
  ├─ Pump.fun Sybil Radar
  ├─ Raydium Pool Guardian
  ├─ Walletless Claim Shield
  ├─ A-F grade
  ├─ risk index
  ├─ evidence summary
  └─ signed rule version
```

---

## Security Rules

Koschei follows strict product boundaries:

```text
Customers cannot alter verdict thresholds.
External projects cannot prompt models to change grades.
AI summaries cannot override deterministic final grade.
Owner controls rule versions and model routing.
Raw prompts, rule weights and God Mode stay private.
```

---

## Local Development

```bash
git clone https://github.com/bugsbuny243/Koschei-Web3-Hub.git
cd Koschei-Web3-Hub/koschei/api
go run main.go
```

Build check:

```bash
go test ./...
go build ./...
```

---

## Project Structure

```text
.
├── README.md
├── Dockerfile
├── db/
└── koschei/
    └── api/
        ├── main.go
        ├── internal/
        │   ├── handlers/
        │   ├── http/
        │   ├── services/
        │   └── db/
        └── public/
            ├── index.html
            ├── dashboard.html
            ├── security-radar.html
            ├── security-ecosystem.html
            ├── security-ecosystem.json
            ├── widget.js
            ├── pricing.html
            ├── reports.html
            └── owner-production.html
```

---

---

Built for Solana security intelligence.
