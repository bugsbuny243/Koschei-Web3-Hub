# Koschei member auth API

`auth-api` is the only server that talks directly to Neon Auth for public member signup and login. The Next.js `/api/auth/*` handlers proxy requests to this service. Admin authentication remains a separate Next.js-only flow based on `ADMIN_EMAIL` and `ADMIN_PASSWORD`.

## Endpoints

- `GET /health`
- `POST /auth/signup`
- `POST /auth/login`
- `GET /auth/me`
- `POST /auth/logout`

Signup and login call Neon Auth's `sign-up/email` and `sign-in/email` endpoints, accept the provider JWT from the `set-auth-jwt` header or common response-body token fields, verify RS256 or EdDSA signatures against the configured JWKS, validate the normalized issuer and expiry, upsert `app_user_profiles(auth_subject, email)`, and issue an httpOnly `koschei_member_session` cookie.

## Configuration

| Variable | Purpose |
| --- | --- |
| `AUTH_API_ADDR` | Optional listen address. Defaults to `:8080`. |
| `APP_ENV` | Set to `production` to add the cookie `Secure` flag. |
| `NEON_AUTH_BASE_URL` | Neon Auth base URL used for email signup and login. |
| `NEON_AUTH_JWKS_URL` | JWKS URL used to verify Neon Auth JWTs. |
| `NEON_AUTH_ISSUER` | Optional explicit JWT issuer. Falls back to `NEON_AUTH_BASE_URL`, then the JWKS origin. |
| `DATABASE_URL` | Neon Postgres connection string used by the Neon SQL HTTP endpoint. |
| `USER_SESSION_SECRET` | Required HMAC secret for member-session cookies. |

Run locally with `go run .` and verify with `curl http://localhost:8080/health`.
