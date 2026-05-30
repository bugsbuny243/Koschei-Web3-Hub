# Koschei API

Core Go backend for Koschei Engine.

## Environment
- DATABASE_URL
- CORS_ALLOWED_ORIGIN
- ADMIN_PASSWORD
- NEON_AUTH_BASE_URL (Auth URL copied from Neon Auth configuration; API base used for `/sign-in/email` and session token follow-up endpoints)
- NEON_AUTH_ISSUER (must match the provider bearer JWT `iss` claim)
- NEON_AUTH_JWKS_URL (must point at the provider signing keys for bearer JWT verification)
- NEON_AUTH_AUDIENCE (optional; omit unless the expected JWT audience is known)
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
- POST /api/auth/login (Neon Auth / Better Auth email + password sign-in via `/sign-in/email`)
- POST /api/auth/otp/start (disabled unless an OTP plugin is enabled and the backend is updated later)
- POST /api/auth/otp/verify (disabled unless an OTP plugin is enabled and the backend is updated later)
- GET /api/me
- POST /api/ai/generate
- POST /api/v1/build/android
- Runtime and artifacts endpoints under /api/runtime and /api/artifacts

## Authentication mode

Koschei delegates production user authentication to Neon Auth / Better Auth email + password. The login handler accepts `email` and `password`, forwards them to the configured provider endpoint at `NEON_AUTH_BASE_URL + /sign-in/email`, and does not store, log, or return the password. After the provider returns a session/JWT/cookie, Koschei only treats three-segment JWT-looking strings as bearer JWT candidates. If `/sign-in/email` returns only an opaque session token or cookie, the backend keeps the provider `Set-Cookie` values in memory for that request and tries provider session/token endpoints (`/token`, `/get-session`, then `/session`) until it finds a verifiable provider-issued bearer JWT. The verified JWT must validate against `NEON_AUTH_JWKS_URL` and `NEON_AUTH_ISSUER`; then Koschei upserts `app_user_profiles` by auth subject and email and returns only `access_token`, `token_type`, and `user` to the browser.

Email OTP routes are intentionally disabled in this backend because the current provider dashboard enables email/password sign-up and sign-in, not the Better Auth OTP plugin. Do not configure the browser to call `/email-otp/send-verification-otp` or `/sign-in/email-otp` unless that plugin is enabled later and the backend is explicitly updated.
