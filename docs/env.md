# Environment Variables (Koschei Phase 3)

This document defines the canonical environment variables for Koschei Phase 3.

## Required production variables

- `DATABASE_URL`: Neon Postgres connection string for the API.
- `CORS_ALLOWED_ORIGIN`: Allowed frontend origin (production is `https://tradepigloball.co`).
- `ADMIN_PASSWORD`: Admin endpoint protection secret.
- `EXPO_PUBLIC_NEON_AUTH_URL`: Public Neon Auth URL used by the Expo frontend.
- `NEON_AUTH_JWKS_URL`: JWKS endpoint used by the Go backend to validate Bearer tokens (including `/api/me`).
- `TOGETHER_API_KEY`: API key for Together model routing.
- `TOGETHER_MODEL`: Default Together model.
- `TOGETHER_MODEL_COMPLEX`: Together model for complex prompts.
- `TOGETHER_MODEL_REASONING`: Together model for reasoning tasks.
- `TOGETHER_MODEL_IMAGE`: Together image generation model.
- `TOGETHER_MODEL_IMAGE_EDIT`: Together image editing model.
- `TOGETHER_MODEL_VIDEO`: Together video model.
- `TOGETHER_MODEL_VIDEO_CINEMA`: Together cinematic video model.
- `TOGETHER_MODEL_TTS`: Together text-to-speech model.
- `TOGETHER_MODEL_STT`: Together speech-to-text model.
- `BRAVE_SEARCH_API_KEY`: Brave Search API key.
- `CLOUDINARY_CLOUD_NAME`: Cloudinary cloud name.
- `CLOUDINARY_API_KEY`: Cloudinary API key.
- `CLOUDINARY_API_SECRET`: Cloudinary API secret.
- `CLOUDINARY_URL`: Cloudinary URL-style credentials string.

## Optional variables

- `NEON_AUTH_ISSUER`: Optional issuer to enforce when validating JWTs from Neon Auth.
- `TOGETHER_ENABLED`: Feature flag for Together routing (default: `true`).
- `TOGETHER_MODEL_*`: Individual model overrides can be swapped per deployment.
- `SEARCH_PROVIDER`: Search provider selector (default: `brave`).
- `MEDIA_PROVIDER`: Media provider selector (default: `cloudinary`).

## Removed legacy variables

The following were removed because they are out-of-scope for Phase 3 or from deprecated auth/escrow experiments:

- Web3/RPC and blockchain variables (e.g. `SOLANA_DEVNET_RPC_URL`, `ZKSYNC_RPC_URL`, `BASE_RPC_URL`, `OPTIMISM_RPC_URL`, `UNICHAIN_RPC_URL`, and sepolia variants).
- Legacy secrets (`KOSCHEI_CREDENTIALS_ENCRYPTION_KEY`, `CRON_SECRET`, `WEBHOOK_SECRET`, `ALCHEMY_WEBHOOK_SIGNING_KEY`).
- Deprecated Neon auth variables (`NEON_AUTH_BASE_URL`, `NEON_AUTH_COOKIE_SECRET`, `NEON_AUTH_AUDIENCE`).
- Deprecated frontend Neon path variables (`EXPO_PUBLIC_NEON_AUTH_BASE_URL`, `EXPO_PUBLIC_NEON_AUTH_SIGNIN_PATH`, `EXPO_PUBLIC_NEON_AUTH_SIGNUP_PATH`).
- Escrow variables (`ESCROW_ENV`, `ESCROW_API_BASE_URL`, `ESCROW_EMAIL`, `ESCROW_API_KEY`, `ESCROW_DEFAULT_SELLER_EMAIL`, `ESCROW_DEFAULT_CURRENCY`, `ESCROW_FEE_PAYER`, `ESCROW_WEBHOOK_TOKEN`).

## Security warning

Any variable prefixed with `EXPO_PUBLIC_` is bundled into the client app and is publicly visible. Never store secrets in `EXPO_PUBLIC_*` variables.
