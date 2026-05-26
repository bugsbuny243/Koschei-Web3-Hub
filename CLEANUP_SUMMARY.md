# Cleanup Summary

## Files/Folders Removed
- `koschei/frontend/` (legacy SaaS/mobile frontend)
- `docs/` (legacy project direction docs)
- `PHASE3_NOTES.md`
- Root Node/Turbo frontend files: `package.json`, `package-lock.json`, `turbo.json`, `tsconfig.base.json`

## Files/Folders Kept
- Go backend: `koschei/api/**`
- Workers: `koschei/workers/**`
- Deployment/build: `Dockerfile`, `railway.toml`, `start.sh`, `koschei/docker-compose.yml`, `koschei/api/Dockerfile`
- DB migrations: `migrations/**`
- New minimal static landing: `public/index.html`

## Env Vars Required Now (runtime/deploy)
- `DATABASE_URL`
- `NEON_AUTH_JWKS_URL` (+ existing Neon Auth envs used in deployment)
- `CORS_ALLOWED_ORIGIN` (if cross-origin frontend is used)
- `ADMIN_PASSWORD` (if owner-protected endpoints are used)
- `GOOGLE_APPLICATION_CREDENTIALS_JSON` (if Google Play publisher flow enabled)
- `ANDROID_PLAY_PACKAGE_NAME`
- `WORKER_MAX_BUILD_THREADS`
- `TOGETHER_API_KEY`
- `TOGETHER_MODEL_GAME_DESIGN`
- `TOGETHER_MODEL_UNREAL_CODE`
- `TOGETHER_MODEL_BUILD_ANALYZER`
- Optional: `TOGETHER_MODEL_CONCEPT_ART`

## Env Vars Removed From Code References
- `TOGETHER_MODEL`
- `TOGETHER_MODEL_COMPLEX`
- `TOGETHER_MODEL_IMAGE`
- `TOGETHER_MODEL_IMAGE_EDIT`
- `TOGETHER_MODEL_REASONING`
- `TOGETHER_MODEL_SECURITY`
- `TOGETHER_MODEL_STT`
- `TOGETHER_MODEL_TTS`
- `TOGETHER_MODEL_VIDEO`
- `TOGETHER_MODEL_VIDEO_CINEMA`
- `TOGETHER_AI_ENABLED`

## Build/Test Command Results
- `go test ./...` (pass)
- `go build ./...` (pass)
- `docker build -t koschei-cleanup .` (pass)
