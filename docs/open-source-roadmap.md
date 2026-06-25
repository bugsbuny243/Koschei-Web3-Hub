# Open Source Roadmap

Koschei ARVIS is moving from a customer dashboard toward Solana-native security infrastructure. The open-source plan focuses on components that can help builders even without using the hosted product.

## Component 1: Event normalization helpers

Normalize launch and liquidity observations into a stable internal format.

Planned scope:

- Pump-style launch event fields
- Raydium-style pool event fields
- target, target type and network normalization
- timestamp and source metadata

## Component 2: Signed verdict schema

Publish the final verdict contract used by radar, reports and API consumers.

Planned scope:

- JSON schema
- example low, medium and critical verdicts
- withheld verdict example
- simple verifier example

## Component 3: SDK starter

A small SDK starter for Solana builders who want to request and display ARVIS verdicts.

Planned scope:

- TypeScript client
- request helpers
- response parser
- error handling examples

## Component 4: Integration examples

Examples for teams that want to embed ARVIS risk intelligence.

Planned examples:

- wallet warning widget
- launchpad screening widget
- research dashboard feed consumer
- webhook alert receiver

## 30 / 60 / 90 day plan

### 0-30 days

- publish signed verdict schema
- publish event normalization draft
- publish API documentation v1

### 30-60 days

- publish SDK starter
- publish integration examples
- add webhook documentation

### 60-90 days

- pilot integrations with Solana builders
- publish case studies and benchmark notes
- expand wallet and token intelligence examples
