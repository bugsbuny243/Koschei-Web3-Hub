# Koschei API (Runtime Phase 1)

## Architecture
- Single-service, same-origin deployment: Go API serves frontend static files and `/api/*` routes from one container/service.
- No separate API service is required in Railway for runtime mode.

## Environment
- `DATABASE_URL` (required)
- `ADMIN_PASSWORD` (required for owner endpoints)
- `CORS_ALLOWED_ORIGIN` (optional)
- `PORT` (optional, default `8080`)

## Run
```bash
go mod tidy
go run main.go
```

## Health
- `GET /health`

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

## Owner endpoints (header `x-admin-password`)
- `GET /api/owner/payment-requests`
- `POST /api/owner/activate-plan`
- `POST /api/owner/grant-credits`
- `PATCH /api/owner/jobs/:id/status`
- `GET /api/owner/db-health`

## Migrations
- `migrations/001_runtime_core.sql`
- `migrations/002_quantum_runtime.sql`
- `migrations/002_runtime_tables.sql`
- `migrations/003_model_routing.sql`
- `migrations/004_rename_builder_to_starter.sql`
- `migrations/005_db_indexes_and_safety.sql`
- `migrations/006_runtime_observability.sql`
- `migrations/007_normalize_runtime_schema.sql`
