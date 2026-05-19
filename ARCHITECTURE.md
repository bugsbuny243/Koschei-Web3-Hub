# Architecture

## Locked MVP Scope
Koschei Web Game Factory + Web3 Bridge only:
- Prompt -> playable HTML5 preview
- Prompt -> Web3-ready package (manifest, metadata, adapter config)
- No-custody boundary (no private keys, no wallet custody, no transactions)

## Runtime
- Frontend/backend: Next.js app in `apps/web`
- Database: Neon Postgres
- Hosting: Railway
- RPC: Alchemy read-only/future integrations only

## Core Flow
1. Create project (`/game-factory/new`)
2. Generate brief/assets/preview
3. View playable preview (`/game-factory/projects/[id]/preview`)
4. Produce Web3-readiness package (`/game-factory/projects/[id]/web3`)

## Safety Boundary
System never manages wallets, keys, minting, contract deploy, or transaction signing.

## Commercial CTA
Shopier support CTA is included in product-facing pages/docs as the current support path.
