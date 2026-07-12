# Production Data Audit Report

Koschei production surfaces must render retrieved evidence, an explicitly partial report, or an honest unavailable state. Demo output, fabricated intelligence and disconnected feature implementations are not production assets.

## Current product rule

- ARVIS is the single risk-intelligence surface.
- Internal evidence arms may remain separate in services, but disconnected customer products are removed.
- A handler is live only when registered in `internal/http/server.go`.
- Git history is the archive for removed experiments; dead implementations are not retained in the production package.
- Applied migrations remain immutable even if the feature that originally created a table has been retired.
- Missing evidence stays `INSUFFICIENT EVIDENCE`; unavailable modules never become LOW by default.

## Removed legacy surfaces

The July 2026 cleanup removed disconnected implementations for:

- old Rug Radar and generic Web3 event-source products
- local email/password authentication and locally signed customer JWTs
- standalone MEV shield and liquidity alert products
- old impact-metric, metadata-generator and smart-money placeholders
- unused generation/Web3 job handlers
- old package, plan, credit and manual payment handlers
- disconnected DAO Guardian and owner payment-health panels

The live replacements are Neon Auth, free core + KOSCH premium access, ARVIS Radar, Security Radar streams, selective Pump 500K+ automation and the owner operations center.

## Verification contract

Every cleanup change must pass:

- focused authentication tests
- production package build
- full Linux build
- route registration assertions
- a source scan proving removed handler symbols are absent

Public pages or documentation must not advertise removed routes.
