# Koschei API

## Architecture
- Single-service deployment: Go API serves static frontend + `/api/*` routes from one container/service.
- Railway-friendly runtime for web game execution, build jobs, and publishing workflows.

## Environment
- `DATABASE_URL` (required)
- `ADMIN_PASSWORD` (required for internal operations)
- `TOGETHER_API_KEY` (required for AI generation)
- `TOGETHER_MODEL_GAME_DESIGN` (required)
- `TOGETHER_MODEL_GAME_CODE` (required)
- `TOGETHER_MODEL_BUILD_ANALYZER` (required)
- `TOGETHER_MODEL_CONCEPT_ART` (optional)
- `CORS_ALLOWED_ORIGIN` (optional)
- `PORT` (optional, default `8080`)

## Run
```bash
go mod tidy
go run main.go
```

## Health
- `GET /health`

## Product Alignment
- Customer enters a game idea and receives a real playable game.
- Web runtime builds are supported.
- Android export supports AAB as primary Play artifact; APK remains optional for testing/download.
- Google Play publishing uses customer-connected account credentials.
- Google Play review and approval remain controlled by Google.

## Runtime/Generation Endpoints
- `GET /api/plans`
- `POST /api/billing/manual-payment-request`
- `GET /api/credits?email=...`
- `GET /api/jobs?email=...`
- `POST /api/jobs`
- `GET /api/runtime/projects?email=...`
- `POST /api/runtime/projects`
- `GET /api/runtime/tasks?email=...`
- `GET /api/runtime/logs/:projectId`

## Internal Operations Endpoints (header `x-admin-password`)
- `GET /api/owner/payment-requests`
- `POST /api/owner/activate-plan`
- `POST /api/owner/grant-credits`
- `PATCH /api/owner/jobs/:id/status`
- `GET /api/owner/db-health`

## Database Naming Direction
- `game_projects`
- `game_templates`
- `game_scenes`
- `game_entities`
- `game_assets`
- `game_build_jobs`
- `game_artifacts`
- `google_play_integrations`
- `production_release_jobs`
