# Koschei Bridge Monorepo

## Project Purpose
Koschei Bridge is a backend-controlled custodial bridge foundation on Base Sepolia. The platform uses invisible server-side wallets where private keys are generated and managed only on the backend.

## Architecture
- **apps/web**: Next.js + TypeScript + Tailwind UI, Auth.js authentication, protected routes, wallet APIs.
- **packages/contracts**: Hardhat smart contracts, deployment scripts, and tests.
- **packages/shared**: shared types and cryptographic helpers used by backend code.
- **prisma**: PostgreSQL schema and migrations.

### Core Security Decisions
- No Supabase, Firebase, or Privy.
- No private key material in frontend code.
- No private key or mnemonic returned in API responses.
- Private keys encrypted before storing in PostgreSQL.
- Server secrets must stay outside `NEXT_PUBLIC_*` variables.

## Setup
```bash
npm install
cp .env.example .env
```

## Required Environment Variables
```env
DATABASE_URL=
AUTH_SECRET=
AUTH_URL=
KOSCHEI_WALLET_ENCRYPTION_KEY=
BASE_SEPOLIA_RPC_URL=
DEPLOYER_PRIVATE_KEY=
BASESCAN_API_KEY=
```

## Database Commands
```bash
npm run prisma:generate
npm run prisma:migrate
```

## Local Development Commands
```bash
npm run dev
npm run build
npm run test
```

## Wallet Security Notes
- `POST /api/wallet/create` requires an authenticated session.
- If a wallet exists, API returns existing `address` + `chain` only.
- If absent, backend generates wallet with `ethers.Wallet.createRandom()`.
- Backend encrypts private key with `KOSCHEI_WALLET_ENCRYPTION_KEY` before DB write.
- `GET /api/wallet/me` returns public wallet metadata only.
- Private keys and mnemonics are never logged or returned.

## Contract Deployment Notes
Contracts live under `packages/contracts` and support Base Sepolia (`chainId: 84532`).

```bash
npm run deploy:base-sepolia -w packages/contracts
npm run test -w packages/contracts
```
