# Koschei

Koschei Game Factory creates the actual customer-requested game and automates publishing to real users through the customer’s own Google Play account.

## Active Product
- Customer-owned web game and Android game creation
- Build automation and Android AAB build artifact generation
- Google Play production rollout automation
- Real user publishing flow through customer-connected Play Console credentials

## Final Customer Flow
1. Customer signs in.
2. Customer enters the game idea.
3. Koschei generates the actual customer-owned game.
4. Koschei creates a playable web version if selected.
5. Koschei creates an Android AAB for Google Play.
6. APK generation is optional for download/testing only, not the primary Play publishing format.
7. Customer connects their own Google Play account / Play Console credentials.
8. Koschei submits the release through the customer’s Google Play account.
9. Default target is production release unless customer explicitly chooses internal/closed testing.
10. The game is intended for real users.

## Publishing Truth
- Koschei automates submission and release preparation.
- Google Play review and approval cannot be bypassed.
- Google controls review outcome, policy approval, and final availability timing.

## Core Platform Ownership
The generated game belongs to the customer. Koschei platform ownership remains with Koschei:
- backend services
- generators and reusable templates
- workers/tooling/infrastructure
- build system and publishing automation

Commercial model target:
- 60% customer / 40% Koschei on net distributable revenue (or equivalent production-cost model)

## Backend and Deploy
- Go backend retained
- Neon Auth integration retained
- PostgreSQL/Neon DB integration retained
- Railway deployment/build retained
- Google Play / GCP publishing integration retained
- Together AI game design/code/build analyzer model integration retained
- Health endpoint: `/health`

## Production Data Model Direction
- `game_projects`
- `game_build_jobs`
- `game_artifacts`
- `google_play_integrations`
- `production_release_jobs`
- `customer_game_ownership`

## Minimal Landing
Static landing page is served from `public/index.html`.
