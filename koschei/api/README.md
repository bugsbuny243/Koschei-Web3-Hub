# Koschei API

Core Go backend for Koschei Engine.

## Environment
- DATABASE_URL
- CORS_ALLOWED_ORIGIN
- ADMIN_PASSWORD
- NEON_AUTH_BASE_URL (Better Auth / Neon Auth URL; usually the Auth URL copied from the Neon Auth configuration)
- NEON_AUTH_ISSUER (issuer expected in provider-issued bearer JWTs)
- NEON_AUTH_JWKS_URL (JWKS used to verify provider-issued bearer JWTs)
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
- POST /api/auth/register (disabled custom auth)
- POST /api/auth/login (Neon Auth / Better Auth email + password sign-in with safe `/sign-in/email` and `/api/auth/sign-in/email` endpoint fallbacks)
- POST /api/auth/otp/start (disabled unless an OTP plugin is enabled and the backend is updated later)
- POST /api/auth/otp/verify (disabled unless an OTP plugin is enabled and the backend is updated later)
- GET /api/me
- POST /api/ai/generate
- POST /api/v1/build/android
- Runtime and artifacts endpoints under /api/runtime and /api/artifacts

## Authentication mode

Koschei delegates production user authentication to Neon Auth / Better Auth email + password. `NEON_AUTH_BASE_URL` should usually be the Auth URL copied from the Neon Auth configuration. The login handler accepts `email` and `password`, first posts to `NEON_AUTH_BASE_URL + /sign-in/email`, and if that provider endpoint returns 404 it safely retries equivalent `/api/auth/sign-in/email` candidates for Auth URLs copied with or without `/api/auth` or `/auth`. It stops on the first non-404 provider response, and does not store, log, or return the password. After the provider returns a session/JWT/cookie, Koschei extracts the provider-issued bearer JWT, verifies it with `NEON_AUTH_JWKS_URL` and `NEON_AUTH_ISSUER`, upserts `app_user_profiles` by auth subject and email, and returns only `access_token`, `token_type`, and `user` to the browser.

Email OTP routes are intentionally disabled in this backend because the current provider dashboard enables email/password sign-up and sign-in, not the Better Auth OTP plugin. Do not configure the browser to call `/email-otp/send-verification-otp` or `/sign-in/email-otp` unless that plugin is enabled later and the backend is explicitly updated.
