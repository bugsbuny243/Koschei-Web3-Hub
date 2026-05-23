# Runtime Phase 1

This phase introduces a separate Go API runtime at `koschei/api` while keeping the existing frontend deploy unchanged.

## What was added
- New Go API service with plans, billing requests, credits, jobs, and owner operations.
- Postgres migration with core runtime tables and plan seeds.
- Frontend API client with graceful fallback when `VITE_API_BASE_URL` is missing.
- New frontend routes:
  - `/pricing`
  - `/billing`
  - `/dashboard`
  - `/owner` (not linked in public navbar)
- Public homepage no longer displays Model Router or raw model names.

## Deployment notes
- Keep existing root Dockerfile unchanged for current frontend deployment.
- Deploy `koschei/api` as a separate Railway service using `koschei/api/Dockerfile`.
