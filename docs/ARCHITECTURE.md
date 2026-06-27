# Architecture Overview

Koschei is a Solana-native risk-intelligence system built around evidence collection, normalization, analysis and verdict delivery.

## High-level flow

```text
Solana RPC and supported program activity
        ↓
transaction and account parsing
        ↓
target resolution
        ↓
evidence normalization
        ↓
ARVIS analysis arms
        ↓
Final Verdict Engine
        ↓
API, dashboard, radar and report surfaces
```

## Core layers

### 1. Evidence collection

The collection layer obtains supported on-chain and off-chain evidence while preserving source boundaries. A URL-derived claim is not represented as on-chain proof, and a program relation is not represented as confirmed wallet coordination without the required graph evidence.

### 2. Target resolution

Raw events are resolved into supported analysis targets such as tokens, wallets, pools, transactions, programs and claim surfaces.

### 3. Evidence analysis

ARVIS evaluates evidence through specialized internal arms covering token authority, holder concentration, liquidity movement, creator relations, funding clusters, transaction execution, MEV exposure and other supported signals.

Each arm remains unsigned when its required evidence is unavailable.

### 4. Verdict production

The Final Verdict Engine produces a customer-facing result only when the evidence boundary permits it. Missing evidence does not silently become a low-risk score.

### 5. Delivery surfaces

Results are exposed through:

- customer dashboard
- live security radar
- signed report vault
- authenticated APIs
- live verdict feed
- public risk badges
- sanitized health endpoint

## Reliability properties

The stream processor is designed to be idempotent: the same stream event and analysis arm must not create duplicate verdicts.

Pipeline health states include:

- `healthy`
- `processing`
- `degraded`
- `stale`
- `waiting_for_stream`
- `waiting_for_enriched_targets`
- `waiting_for_processing`

## Data layer

Neon Postgres stores radar events, processing jobs, verdicts, recovery state and idempotency constraints. Database migrations are the source of truth for schema evolution.

## Runtime boundary

The production deployment environment supplies credentials and provider configuration. Secrets are not committed to the repository.

## External integration boundary

The public integration layer should expose stable, versioned schemas while keeping internal scoring heuristics and proprietary verdict logic behind the service boundary. Open-source components should focus on reusable ecosystem primitives such as decoding, schema helpers, examples and integration starters.
