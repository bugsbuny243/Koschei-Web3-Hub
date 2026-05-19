# Koschei Web Game Factory + Web3 Bridge MVP

Koschei converts a plain-language game prompt into a playable browser preview and a no-custody Web3-ready package.

## Locked MVP flow
1. User creates a game project from prompt.
2. Project is saved to Neon Postgres.
3. Game preview is generated.
4. Project detail page shows generated outputs.
5. Web3 package is generated.
6. Projects list works.
7. Shopier support CTA remains visible.

## Product scope
- Koschei Web Game Factory
- Koschei Web3 Bridge (no-custody)
- Prompt-to-game generation
- Preview generation (canvas HTML)
- Web3-ready package (manifest, item schemas, NFT-compatible metadata, reward config, adapter config)

## Safety
Koschei Web3 Bridge MVP does not hold funds, manage private keys, connect wallets, deploy contracts, mint NFTs, sign transactions, or custody user assets. It only prepares game manifests, item schemas, NFT-compatible metadata, reward configs, and adapter configs.

## Infrastructure
- App runtime: Next.js
- Database: Neon Postgres
- Deployment: Railway
- Web3 RPC/reads: Alchemy read-only/future config
- Support: Shopier CTA

## Development
```bash
npm install
npm run build -w apps/web
npm run start -w apps/web
```
