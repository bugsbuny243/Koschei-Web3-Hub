# Koschei API

Core Go backend for Koschei Engine.

## Environment
- DATABASE_URL
- CORS_ALLOWED_ORIGIN
- ADMIN_PASSWORD
- NEON_AUTH_BASE_URL
- NEON_AUTH_ISSUER
- NEON_AUTH_JWKS_URL
- NEON_AUTH_AUDIENCE (optional)
- EXPO_PUBLIC_NEON_AUTH_URL (static/frontend hint)
- TOGETHER_API_KEY
- TOGETHER_MODEL_GAME_DESIGN
- TOGETHER_MODEL_GAME_CODE
- TOGETHER_MODEL_BUILD_ANALYZER
- TOGETHER_MODEL_CONCEPT_ART
- GOOGLE_APPLICATION_CREDENTIALS_JSON
- ANDROID_PLAY_PACKAGE_NAME
- WORKER_MAX_BUILD_THREADS

## Main Endpoints
- GET /health
- POST /api/auth/register
- POST /api/auth/login (disabled custom auth)
- POST /api/auth/otp/start
- POST /api/auth/otp/verify
- GET /api/me
- POST /api/ai/generate
- POST /api/v1/build/android
- Runtime and artifacts endpoints under /api/runtime and /api/artifacts
