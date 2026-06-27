# Koschei ARVIS — Solana Technical Infrastructure Grant Resubmission

## Project name

Koschei ARVIS — Evidence-Backed Solana Risk Infrastructure

## One-line summary

Koschei ARVIS is a Solana-native security infrastructure layer that converts live launch, liquidity, token, wallet and transaction observations into versioned evidence and signed risk verdicts for wallets, launchpads, developer tools and security teams.

## Response to the previous review

The previous application was interpreted as community, education and ecosystem-promotion work. This resubmission removes that ambiguity.

The funded scope is engineering work only:

- production Solana data ingestion and normalization
- deterministic evidence processing
- signed verdict infrastructure
- open-source developer packages
- API and SDK delivery
- integration pilots with Solana builders

Community events, general education and promotional activity are not grant deliverables.

## Problem

Solana users and builders face fast-moving risk around new tokens, liquidity pools, suspicious wallets, transaction routes and claim surfaces. Existing workflows often require manual checks across several tools, while many risk scores do not expose enough evidence for downstream applications to trust or audit them.

Builders need a reusable technical layer that:

1. normalizes live Solana observations
2. separates verified evidence from assumptions
3. produces a stable machine-readable verdict
4. withholds output when evidence is insufficient
5. can be embedded through APIs and open-source clients

## Current technical system

Koschei ARVIS currently includes:

- a Go API and worker runtime
- Neon PostgreSQL event, processing and verdict stores
- Solana RPC enrichment with provider failover
- Pump-style launch observation ingestion
- Raydium-oriented liquidity and program analysis
- idempotent processing and retry states
- evidence-backed final verdict generation
- authenticated partner APIs
- a live radar and report delivery layer

## Shipped open-source outputs

### TypeScript SDK

Location: `sdk/typescript`

Provides dependency-free clients for current production routes, structured API errors and deterministic signed-verdict validation.

### Signed verdict CLI

Location: `sdk/typescript/bin/arvis-verdict.mjs`

Validates verdict JSON from a file or stdin and returns deterministic exit codes for accepted, malformed and withheld results.

### Solana event normalizer

Location: `oss/event-normalizer`

Converts Pump-style and Raydium-style observations into a versioned event contract. Invalid timestamps, missing targets and unclassified sources are rejected instead of silently converted into evidence.

### Signed verdict schema

Location: `oss/schemas/signed-verdict.schema.json`

Defines the integration contract for grades, risk indexes, risk levels, evidence, rule versions and signed status.

### Integration examples

Location: `examples`

Includes wallet-warning and launchpad-screening examples for Solana builders.

### Technical documentation

The repository includes API references, a developer quickstart, data-flow architecture, signed-verdict documentation, limitations and a technical whitepaper.

## Grant-funded technical deliverables

### Milestone 1 — Reproducible developer releases

- publish versioned npm releases for the SDK and event normalizer
- publish deterministic fixtures for Pump and Raydium observations
- publish package checksums and release notes
- document compatibility and support policy

Acceptance criteria:

- clean installation on Node.js 18+
- strict type-check and runtime tests pass in CI
- example input produces documented deterministic output
- package archives contain only documented distributable files

### Milestone 2 — Solana integration surfaces

- add a wallet integration starter
- add a launchpad token-screening starter
- add batch screening support
- finalize webhook delivery and receiver examples

Acceptance criteria:

- each starter uses a documented live API route
- signed and withheld outcomes are handled separately
- credentials are never included in client bundles or examples

### Milestone 3 — Evidence and reliability expansion

- expand token-authority, liquidity and wallet relation evidence
- add provider-failover telemetry
- publish benchmark fixtures and expected rule outputs
- document rule-version governance and false-positive review

Acceptance criteria:

- every final verdict identifies its rule version
- missing required evidence produces a withheld result
- provider outages do not silently generate clean verdicts

### Milestone 4 — Ecosystem pilots

- complete pilot integrations with Solana builders
- collect integration reliability and latency data
- publish anonymized technical case studies
- convert pilot feedback into SDK and schema improvements

Acceptance criteria:

- at least two external integration pilots
- published integration notes and reproducible examples
- documented changes produced from pilot feedback

## Ecosystem impact

Koschei ARVIS can provide reusable infrastructure for:

- wallets that need pre-signing risk warnings
- launchpads that need token and creator screening
- research products that need structured evidence bundles
- security operations teams that need live monitoring
- discovery products that need explainable risk labels

The open-source packages remain useful even when a builder does not use the hosted dashboard.

## Engineering quality and safety

- Go API changes run tests, vet and build checks.
- Open-source TypeScript packages run strict compilation, runtime tests and package validation.
- The same event and evidence arm cannot create duplicate final output.
- AI explanations do not override deterministic evidence.
- A missing evidence set produces no signed final verdict.
- The project does not claim guaranteed safety or provide investment advice.

## Funding use

Grant funding will be used for:

- 55% engineering and open-source package delivery
- 20% production infrastructure and Solana data access
- 15% integration pilots and developer support
- 10% technical documentation, benchmarks and release operations

## Why this fits the program

This scope delivers technical products, open-source software, developer resources and long-term Solana-native infrastructure. The project is no longer presented as a community or education initiative; the measurable outputs are code, schemas, APIs, tested packages, integration starters and production reliability work.
