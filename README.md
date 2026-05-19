# Koschei Web Game Factory + Web3 Bridge (Locked MVP)

Koschei is a **prompt-to-playable web game factory** with a **no-custody Web3-ready packaging flow**.

## Locked MVP product flow
1. User submits a game prompt.
2. API creates a structured game brief.
3. System generates a playable HTML5 preview.
4. System extracts game items/rewards/achievements.
5. System generates NFT-compatible metadata.
6. System generates a Web3-ready package with:
   - game manifest
   - item schema
   - NFT metadata
   - reward config
   - quest/achievement config
   - Arbitrum Sepolia adapter config

## No-custody safety scope
This MVP is strictly no-custody:
- no MetaMask / WalletConnect connect flow
- no `window.ethereum`
- no transaction signing
- no private key handling
- no contract deployment
- no minting
- no escrow or funds movement

## Stack
- Next.js App Router (apps/web)
- Neon Postgres (via Prisma + PG queries)
- Railway deployment target
- Alchemy read-only chain monitoring/config surface (future extension)

## Core routes
- `/game-factory/new` create project and run full generation pipeline
- `/game-factory/projects` list projects
- `/game-factory/projects/[id]` project detail + generation actions
- `/game-factory/projects/[id]/preview` playable web preview
- `/game-factory/projects/[id]/web3` Web3 package JSON outputs

## Supporting routes kept active
- PayWatch routes (`/web3`, `/web3/invoices`, scan/webhook APIs)
- Game Bridge pages (`/web3/game-bridge/...`)
- Grant pages (`/web3/grant`, `/web3/game-bridge/grant`)
- Shopier support CTA in global layout footer

## Demo flow
1. Open `/game-factory/new`
2. Enter prompt and submit
3. Wait for project creation + game generation + web3 package generation
4. Land on preview page
5. Open Web3 package page to copy JSON blocks

## Local development
```bash
npm install
npm run dev
```

## Build
```bash
npm run build -w apps/web
```
