# Koschei API (Runtime Phase 1)

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

## Endpoints
- `GET /health`
- `GET /api/plans`
- `POST /api/billing/manual-payment-request`
- `GET /api/credits?email=...`
- `GET /api/jobs?email=...`
- `POST /api/jobs`
- Owner (header `x-admin-password`):
  - `GET /api/owner/payment-requests`
  - `POST /api/owner/activate-plan`
  - `POST /api/owner/grant-credits`
  - `PATCH /api/owner/jobs/:id/status`

## Migrations
SQL migration is in `migrations/001_runtime_core.sql`.
