# Koschei API

Koschei is a Go backend that serves its static frontend and API from one runtime application.

## Architecture

- The runtime application lives in `koschei/api`.
- Static frontend files live in `koschei/api/public` and are served by the Go HTTP server.
- Neon Auth signup, login, token verification, and authenticated API access are handled by the Go backend.
- PostgreSQL data is configured through `DATABASE_URL`.

## Local Development

Copy the environment template and run the Go application:

```bash
cp .env.example .env
cd koschei/api
go run .
```

The server listens on `PORT`, which defaults to `8080`. Open [http://localhost:8080](http://localhost:8080).

## Environment Variables

The deployment template is documented in `.env.example`. Important values include:

- `PORT` and `STATIC_DIR` for the HTTP server and static frontend directory.
- `DATABASE_URL` for PostgreSQL.
- `EXPO_PUBLIC_NEON_AUTH_URL`, `NEON_AUTH_BASE_URL`, `NEON_AUTH_ISSUER`, and `NEON_AUTH_JWKS_URL` for Neon Auth.
- `USER_SESSION_SECRET` for authenticated sessions.
- `ALCHEMY_API_KEY` for Web3 health checks.
- `TOGETHER_API_KEY` and `TOGETHER_MODEL` for AI generation.

## API Endpoints

The Go backend includes these public endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Service health |
| `GET` | `/api/config` | Browser-safe runtime configuration |
| `POST` | `/api/auth/provision` | Provision an authenticated member |
| `POST` | `/api/auth/register` | Existing member registration route |
| `POST` | `/api/auth/login` | Existing Neon Auth login route |
| `GET` | `/api/web3/health` | Web3 provider health |

Additional authenticated API endpoints are registered by the Go server under `koschei/api/internal/http`.

## Railway Deployment

Railway uses the repository-root `Dockerfile`. The container build compiles `koschei/api`, copies `koschei/api/public` into `/app/public`, sets `STATIC_DIR=/app/public`, and starts the compiled Go binary on port `8080`.

Configure the required values from `.env.example` in Railway Variables before deployment. `APP_ENV=production` is required on Railway so production checks are enabled and local-only debug routes stay unavailable. Neon Auth is handled directly by the Go backend.

## Validation

Run the Go build check from the backend directory:

```bash
cd koschei/api
go build ./...
```

Build the production container from the repository root:

```bash
docker build .
```
