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

## Ownership and Commercial Model
- Customer owns each generated game.
- Koschei owns the engine, platform, templates, generator, exporters, workers, and publishing automation.
- Commercial model target: support production fee + optional 60% customer / 40% Koschei net revenue share.

## Product Boundaries
- Focus: simple/medium web games, Android games, fast publishing, AAB generation, customer ownership, Google Play automation.
- Do not claim AAA graphics, heavyweight-console-level rendering, or one-prompt massive open-world generation.
