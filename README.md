# Koschei Engine

Koschei Engine is a lightweight AI-assisted web and Android game engine for customer-owned games.

## Positioning
Koschei Engine is built to replace legacy heavyweight engine complexity for small and medium game production by making creation faster, simpler, cheaper, and publishing-focused.

## Core Customer Promise
1. The customer enters a game idea.
2. Koschei Engine generates a real playable game.
3. The game can run on the web.
4. The game can be exported as an Android AAB.
5. The customer connects their own Google Play account.
6. Koschei publishes/releases through the customer account.

## Product Modules
1. `koschei-runtime` — runtime that executes generated games.
2. `koschei-editor` — simple visual/prompt editor for scenes, sprites, levels, enemies, score, UI, and game rules.
3. `koschei-templates` — starter templates: runner, platformer, clicker, puzzle, quiz, arcade shooter.
4. `koschei-ai-generator` — converts prompts into specs, scenes, entities, rules, code, and asset metadata.
5. `koschei-web-exporter` — exports playable web builds.
6. `koschei-android-exporter` — packages Android APK/AAB artifacts.
7. `koschei-play-publisher` — publishes AAB releases via customer-connected Google Play accounts.

## Platform Scope (Kept)
- Go backend
- Neon Auth
- PostgreSQL / Neon DB
- Railway deploy/build config
- Google Play / GCP publishing integration
- Together AI integration

## Together AI Environment Variables
- `TOGETHER_API_KEY`
- `TOGETHER_MODEL_GAME_DESIGN`
- `TOGETHER_MODEL_GAME_CODE`
- `TOGETHER_MODEL_BUILD_ANALYZER`
- Optional later: `TOGETHER_MODEL_CONCEPT_ART`

## Database Concepts
- `game_projects`
- `game_templates`
- `game_scenes`
- `game_entities`
- `game_assets`
- `game_build_jobs`
- `game_artifacts`
- `google_play_integrations`
- `production_release_jobs`
- `game_store_metadata` (Google Play AI Discovery Pack per generated game spec)

## Ownership and Commercial Model
- Customer owns each generated game.
- Koschei owns the engine, platform, templates, generator, exporters, workers, and publishing automation.
- Commercial model target: support production fee + optional 60% customer / 40% Koschei net revenue share.

## Product Boundaries
- Focus: simple/medium web games, Android games, fast publishing, AAB generation, customer ownership, Google Play automation.
- Do not claim AAA graphics, heavyweight-console-level rendering, or one-prompt massive open-world generation.


## Web3 Bridge

Koschei Web3 Bridge adds a no-custody, read-only event monitoring dashboard for grant/demo work and production-safe developer tooling. The MVP foundation is complete, and production hardening has started with source-scoped shared-secret webhook verification, payload hashing, and event source management APIs. Open the dashboard at [/web3-bridge.html](/web3-bridge.html). See [No-Custody Architecture](docs/web3/NO_CUSTODY_ARCHITECTURE.md) for the safety boundaries: no private keys, no custody, no escrow, and no automatic transfers. See [Production Webhook Security](docs/web3/PRODUCTION_WEBHOOK_SECURITY.md) for webhook setup, source IDs, secret rotation, and Railway environment reminders. See [Solana Integration Guide](docs/web3/SOLANA_INTEGRATION_GUIDE.md) for the production Solana rollout: read-only first, devnet before mainnet, Railway-only RPC secrets, and Alchemy webhook headers.

## Grant / Funding Sprint

### Real Koschei SaaS Pricing (Shopier)
1. **Koschei Starter** — **899 TL** — **20,000 credits**  
   https://www.shopier.com/TradeVisual/47465449
2. **Koschei Pro** — **2,299 TL** — **70,000 credits**  
   https://www.shopier.com/TradeVisual/47465484
3. **Koschei Studio** — **4,999 TL** — **180,000 credits**  
   https://www.shopier.com/TradeVisual/47465499

**Compliance wording:** These are Koschei SaaS credit/support packages. They are not investment products, not securities, not token sales, do not promise profit sharing, and are service/credit/support packages only.

- [Koschei Web3 Bridge Grant Application](docs/grants/KOSCHEI_WEB3_BRIDGE_GRANT_APPLICATION.md)
- [Superteam Solana Microgrant Draft](docs/grants/SUPERTEAM_SOLANA_MICROGRANT_DRAFT.md)
- [Alchemy Credit Application Draft](docs/grants/ALCHEMY_CREDIT_APPLICATION_DRAFT.md)
- [Shopier Early Support Product](docs/grants/SHOPIER_EARLY_SUPPORT_PRODUCT.md)
- [Demo Video Script](docs/grants/DEMO_VIDEO_SCRIPT.md)
