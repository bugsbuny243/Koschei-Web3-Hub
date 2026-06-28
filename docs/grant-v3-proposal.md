# Koschei ARVIS — Solana Technical Infrastructure Grant Resubmission

## 30-second pitch

Koschei ARVIS stops risky Solana interactions before users sign. Wallets, dApps, launchpads, DeFi protocols and security teams call one API and receive an **allow, warn, block or withhold** decision backed by verified evidence, rule metadata and signed status.

Instead of building separate token, wallet, transaction, monitoring and alert systems, Solana product teams integrate one reusable risk layer.

## What changed after the previous review

The previous application was interpreted as community, education and ecosystem-promotion work. This proposal is engineering-only:

- production Solana data ingestion and normalization
- deterministic evidence processing
- pre-signing transaction analysis
- signed verdict infrastructure
- open-source SDK, schemas and integration examples
- external pilots with Solana builders

Community events and promotional activity are not grant deliverables.

## Problem

Solana product teams often combine manual checks and disconnected tools before allowing a token listing, transaction, wallet interaction or monitored target. Many risk scores also hide the evidence needed for a downstream product to trust or audit the result.

ARVIS provides one reusable decision layer that:

1. understands Solana instructions and account relationships
2. normalizes token, launch, liquidity, wallet and program observations
3. separates verified evidence from assumptions
4. returns a stable machine-readable decision
5. withholds a signed verdict when required evidence is unavailable
6. integrates through APIs, SDKs, webhooks and examples

## Who pays — and why

| Customer | Need | Commercial reason |
| --- | --- | --- |
| Wallets | Pre-signing warnings | Reduce preventable risky interactions and support incidents |
| Launchpads | Token and authority screening | Avoid maintaining several separate screening systems |
| dApps and DeFi | Transaction simulation and policy | Apply consistent decisions before execution |
| Security teams | Persistent monitoring | Replace manual monitoring with signed alerts |

Commercial access can be output-based API capacity, monitoring capacity or a B2B integration agreement.

## Why Solana

ARVIS is designed around:

- Solana transaction instructions and account relationships
- SPL Token and Token-2022 authorities and extensions
- mint and freeze authority evidence
- program-specific relations and liquidity activity
- priority-fee and pre-signing simulation context
- Pump-style launch observations
- Raydium-oriented liquidity evidence

It is not a generic chain score with a Solana label.

## What is live

- Go API and worker runtime
- Neon PostgreSQL processing, usage and verdict stores
- Solana RPC enrichment with provider fallback
- production radar and evidence-backed checks
- Token-2022 security scanning
- pre-signing transaction simulation
- persistent watchlists and change alerts
- HMAC-signed webhook delivery with retries and dead-letter handling
- authenticated B2B batch screening with idempotency
- asynchronous result lookup and usage accounting
- TypeScript client, signed-verdict schema and integration examples

## Evidence policy

```text
verified required evidence exists  -> signed verdict may be produced
verified required evidence missing -> final signed verdict is withheld
```

Provider failure does not become a fabricated clean result. AI explanations never override deterministic evidence.

## Three measurable milestones

### 1. Pilot-ready integration release

Deliver versioned SDK and normalizer releases, deterministic fixtures, a wallet starter, a launchpad starter and a webhook verification example.

Acceptance criteria:

- clean installation on Node.js 18+
- strict checks and tests pass in CI
- example input produces deterministic documented output
- signed and withheld outcomes are handled separately
- credentials are absent from client bundles and examples

### 2. External Solana integration pilots

Complete at least two pilots using a real wallet, dApp, launchpad, DeFi or security workflow.

Acceptance criteria:

- named integration owner and documented use case
- documented live API route used
- decision latency and completed-check rate measured
- false-positive and withheld-output samples reviewed
- anonymized technical integration notes published with permission
- pilot feedback produces a documented product change

### 3. Reliability and evidence benchmark

Publish provider-failover telemetry, deterministic benchmark fixtures, rule-version governance and webhook recovery tests.

Acceptance criteria:

- every final verdict identifies its rule version
- missing required evidence produces a withheld result
- provider outages cannot silently generate clean verdicts
- benchmark inputs and expected outputs are public
- webhook retry and dead-letter behavior is reproducible

## Current gap and next proof

The technical system and developer surfaces are live. The next proof is external adoption: integration pilots, measured reliability and published technical case studies.

A pilot application flow at `/pilot` collects the project, integration owner, Solana use case, expected volume and measurable success criteria.

## Funding use

- 55% engineering and open-source package delivery
- 20% production infrastructure and Solana data access
- 15% integration pilots and developer support
- 10% technical documentation, benchmarks and releases

## Program fit

The outputs are code, schemas, APIs, tested packages, integration starters, external pilots and production reliability evidence. This is technical Solana infrastructure, not a community or education program.