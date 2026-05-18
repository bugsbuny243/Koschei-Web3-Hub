# Koscei Bridge (Turborepo MVP)

Koscei Bridge, kullanıcıların doğal dil prompt'larıyla DeFi odaklı AI agent'lar oluşturmasını hedefleyen bir monorepo MVP projesidir. Bu sürümde temel amaç:
- Web arayüzünden “Create your AI Agent” prompt'unu almak,
- `agent-core` içinde LangGraph tabanlı bir workflow çalıştırmak,
- Cüzdan/price/yield/swap araçlarını bir agent katmanında birleştirmek,
- `turbo dev` ile geliştirme ortamını tek komutta ayağa kaldırmaktır.

## Tech Stack

- **Monorepo:** Turborepo + npm workspaces
- **Frontend:** Next.js 14 (App Router), React, TypeScript
- **Wallet UX:** wagmi + RainbowKit + WalletConnect
- **Agent Core:** TypeScript, LangGraph, LangChain, Groq
- **EVM Integration:** viem (Arbitrum RPC), alchemy endpoints
- **Data/Infra:** Prisma, Postgres, Pinata (IPFS)

## Repository Yapısı

- `apps/web`: Next.js web uygulaması
- `packages/agent-core`: Agent sınıfları, tools ve LangGraph workflow
- `packages/contracts`: Solidity + Hardhat
- `packages/shared`: Ortak tip ve yardımcılar

## Kurulum

1. Bağımlılıkları kur:
   ```bash
   npm install
   ```

2. Environment dosyasını oluştur:
   ```bash
   cp .env.example .env
   ```
   Ardından `.env` dosyasına gerçek API key/secret değerlerini gir.

3. (Opsiyonel) Prisma setup:
   ```bash
   npm run prisma:generate
   npm run prisma:migrate
   ```

4. Geliştirme ortamını başlat:
   ```bash
   npm run dev
   ```
   > Bu komut turbo üzerinden `apps/web` ve `packages/agent-core` dev süreçlerini birlikte başlatır.

## MVP Akışı (Grant Odaklı)

### Hedef 1 — Prompt-to-Agent Deneyimi
- Kullanıcı ana sayfada prompt girer: *“Create your AI Agent”*.
- Web uygulaması backend API’ye agent oluşturma isteği yollar.

### Hedef 2 — Agent Tooling
`AutoYieldOptimizerAgent` aşağıdaki tool'ları orkestre eder:
- `getWalletBalance`
- `getTokenPrice` (Pyth / Chainlink fallback)
- `suggestBestYield` (Aave / Compound benzeri mock strateji)
- `executeSwap` (Arbitrum üzerinde viem üzerinden demo execution)

### Hedef 3 — Wallet-first UX
- RainbowKit ile wallet bağlantısı
- WalletConnect Project ID ile çoklu cüzdan desteği

### Hedef 4 — Çalıştırılabilir Monorepo
- Tutarlı TypeScript config
- optimize `turbo.json` pipeline (`dev`, `build`, `lint`)
- Tek komutla çalışma: `npm run dev`

## Scripts

Kök dizin:

- `npm run dev` — web + agent-core dev
- `npm run build` — tüm workspace build
- `npm run lint` — tüm workspace lint
- `npm run test` — mevcut test scriptleri

## Notlar

- Bu MVP’de bazı DeFi entegrasyonları “production-safe execution” yerine “hackathon/demo-safe” yaklaşımla hazırlanmıştır.
- Gerçek swap/yield işlemleri için ek güvenlik kontrolleri, simulation ve izin katmanları şarttır.
