# Koschei Engine

Koschei Engine is a lightweight AI-assisted game creation engine for customer-owned web and Android games.

## Product Direction
- Customers write a game idea in natural language.
- Koschei converts it into a playable web game and/or Android game.
- Koschei generates APK/AAB artifacts.
- Koschei publishes through the customer's connected Google Play account.

## Required Environment Variables
- ADMIN_PASSWORD
- DATABASE_URL
- CORS_ALLOWED_ORIGIN
- NEON_AUTH_BASE_URL
- NEON_AUTH_ISSUER
- NEON_AUTH_JWKS_URL
- GOOGLE_APPLICATION_CREDENTIALS_JSON
- ANDROID_PLAY_PACKAGE_NAME
- WORKER_MAX_BUILD_THREADS
- TOGETHER_API_KEY
- TOGETHER_MODEL_GAME_DESIGN
- TOGETHER_MODEL_GAME_CODE
- TOGETHER_MODEL_BUILD_ANALYZER
- TOGETHER_MODEL_CONCEPT_ART

## Deployment
- Railway (`railway.toml`)
- Docker (`Dockerfile`)
- Health check endpoint: `GET /health`
