# Koschei ARVIS — Solana Pre-Signing Risk Infrastructure

## 30-second pitch

Koschei ARVIS stops risky Solana interactions before users sign. Wallets, dApps, launchpads, DeFi protocols and security teams call one API and receive a machine-readable **allow, warn, block or withhold** decision backed by verified evidence, rule metadata and signed status.

Instead of building separate token, wallet, transaction, monitoring and alert systems, product teams integrate one Solana-native risk layer.

## Who pays — and why

| Customer | What ARVIS replaces or improves | Why they pay |
| --- | --- | --- |
| Wallets | Manual or fragmented pre-signing checks | Reduce preventable risky interactions and support incidents |
| Launchpads | Separate token, authority and concentration checks | Screen assets before listing and preserve an auditable decision record |
| dApps and DeFi protocols | Ad-hoc transaction simulation and policy code | Apply consistent allow, warn, block or withhold policy before execution |
| Security and research teams | Manual monitoring across disconnected tools | Receive persistent evidence-backed alerts and signed webhook delivery |

Commercial access can be sold as output-based API capacity, persistent monitoring capacity or a B2B integration agreement.

## Why Solana

ARVIS is not a generic Web3 score with a Solana label. It is designed around:

- Solana transaction instructions and account relationships
- SPL Token and Token-2022 authorities, extensions and transfer behavior
- mint and freeze authority evidence
- program-specific relations and liquidity activity
- priority-fee and pre-signing simulation context
- Pump-style launch observations and Raydium-oriented liquidity evidence

The system withholds a final signed verdict when required evidence is unavailable. Provider failure does not become a fabricated clean score.

## Product

The customer receives one structured verdict containing:

- decision: allow, warn, block or withhold
- grade and risk index where evidence supports them
- normalized evidence records
- rule version and evaluation metadata
- signed or withheld status
- recommended integration action

The internal evidence arms are verification layers, not separate products.

## What is live

- Go API and worker runtime
- Neon PostgreSQL processing, usage and verdict stores
- Solana RPC enrichment with provider fallback
- production radar and evidence-backed checks
- Token-2022 security scanning
- pre-signing transaction simulation
- persistent watchlists and change alerts
- HMAC-signed webhook delivery with retries and dead-letter handling
- authenticated B2B batch screening with idempotency and asynchronous result lookup
- TypeScript client, signed-verdict schema and integration examples

## Evidence policy

```text
verified required evidence exists  -> signed verdict may be produced
verified required evidence missing -> withhold the final signed verdict
```

AI-generated explanations never override deterministic evidence. ARVIS does not claim guaranteed safety and does not provide investment advice.

## Current gap

The technical system is live. The next proof is external adoption: integration pilots, measured production reliability and published technical case studies.

## Three measurable milestones

### 1. Pilot-ready integration release

Deliver versioned SDK and schema releases, reproducible fixtures, a wallet integration starter and a launchpad screening starter.

Acceptance criteria:

- clean installation and documented live-route usage
- deterministic fixture output in CI
- signed and withheld outcomes handled separately
- no credentials included in browser bundles or examples

### 2. External integration pilots

Complete at least two pilots with Solana builders using a real wallet, dApp, launchpad, DeFi or security workflow.

Acceptance criteria:

- named integration owner and documented use case
- measured decision latency and completed-check rate
- reviewed false-positive and withheld-output samples
- anonymized technical integration notes published with permission

### 3. Reliability and evidence benchmark

Publish rule-version governance, provider-failover telemetry and deterministic benchmark fixtures for token, transaction and monitoring flows.

Acceptance criteria:

- every final verdict identifies its rule version
- provider outages cannot silently generate clean verdicts
- webhook retry and dead-letter behavior is reproducible
- benchmark inputs and expected outputs are public

## Ecosystem impact

ARVIS gives Solana builders a reusable security primitive rather than another isolated dashboard. More integrations create a shared, versioned verdict contract that wallets, launchpads, dApps, research products and security operations can consume consistently.

## Pilot call to action

Technical integration requests are collected at `/pilot`. A strong pilot has one real Solana flow, one accountable integration owner, one explicit risk decision and measurable success criteria.