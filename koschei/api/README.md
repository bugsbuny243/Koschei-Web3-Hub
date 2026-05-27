# Koschei API (Runtime Phase 1)

## Architecture
- Single-service, same-origin deployment: Go API serves frontend static files and `/api/*` routes from one container/service.
- No separate API service is required in Railway for runtime mode.

## Environment
- `DATABASE_URL` (required)
- `ADMIN_PASSWORD` (required for internal admin operations)
- `CORS_ALLOWED_ORIGIN` (optional)
- `PORT` (optional, default `8080`)

## Run
```bash
go mod tidy
go run main.go
```

## Health
- `GET /health`

## Product Goal Alignment
- Runtime flow supports customer-owned game generation and real user publishing.
- Android publishing target is Google Play production release by default.
- AAB is the primary Play artifact; APK may be produced as an optional testing/download artifact.
- Google Play review/approval remains controlled by Google.

## Public runtime endpoints
- `GET /api/plans`
- `POST /api/billing/manual-payment-request`
- `GET /api/credits?email=...`
- `GET /api/jobs?email=...`
- `POST /api/jobs`
- `GET /api/runtime/projects?email=...`
- `POST /api/runtime/projects`
- `GET /api/runtime/tasks?email=...`
- `GET /api/runtime/logs/:projectId`

## Internal admin endpoints (header `x-admin-password`)
- `GET /api/owner/payment-requests`
- `POST /api/owner/activate-plan`
- `POST /api/owner/grant-credits`
- `PATCH /api/owner/jobs/:id/status`
- `GET /api/owner/db-health`

## Production Data Model Naming Direction
- `game_projects`
- `game_build_jobs`
- `game_artifacts`
- `google_play_integrations`
- `production_release_jobs`
- `customer_game_ownership`

## Migrations
- `migrations/001_runtime_core.sql`
- `migrations/002_quantum_runtime.sql`
- `migrations/002_runtime_tables.sql`
- `migrations/003_model_routing.sql`
- `migrations/004_rename_builder_to_starter.sql`
- `migrations/005_db_indexes_and_safety.sql`
- `migrations/006_runtime_observability.sql`
- `migrations/007_normalize_runtime_schema.sql`
