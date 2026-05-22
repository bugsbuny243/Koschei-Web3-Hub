# Railway No-Next Deploy Fix

## Problem
Railway was deploying the legacy `apps/web` Next.js skeleton, which produced a plain-text public page instead of the Koschei platform landing experience.

## Fix Summary
- Root Directory remains `/`.
- Railway build switched to Dockerfile mode.
- Root `Dockerfile` now builds from `koschei/frontend`.
- Root `package.json` no longer points to `apps/web` Next.js workspace scripts.
- `railway.toml` no longer uses Nixpacks Next.js build/start commands.
- Public homepage removes **Enter God Mode** and keeps owner controls private under `/owner`.

## Required Railway Settings
`railway.toml` should be:

```toml
[build]
builder = "DOCKERFILE"
dockerfilePath = "./Dockerfile"

[deploy]
restartPolicyType = "ON_FAILURE"
restartPolicyMaxRetries = 10
```

## Docker Build Path
The deploy container builds the frontend from:

- `koschei/frontend/package*.json`
- `npm ci`
- `npm run build`
- runtime preview via `vite preview --host 0.0.0.0 --port 8080`

## Public vs Owner Visibility
- Public homepage includes: hero, AI tools, model router, SaaS explanation, pricing preview, CTA.
- Public users do **not** see owner links or owner actions.
- Owner-only tools are routed under `/owner` and should stay role-protected.
