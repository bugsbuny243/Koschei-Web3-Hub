# Koschei ARVIS Production Route Map

This document tracks routes registered by `koschei/api/internal/http/server.go`.
A handler is production-live only when it is present in this boot chain.

## Core

- `GET /health`
- `GET /api/config`
- `POST /api/auth/provision`
- `GET /api/web3/health`
- `GET /api/web3/health/logs`
- `POST /api/analytics/event`
- `GET /api/version`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/auth/neon-login`
- `GET /api/auth/neon-register`
- `GET /api/auth/neon-callback`
- `GET /api/me`
- `POST /api/arvis/preflight`
- `GET /api/public/impact`
- `GET /api/public/metrics`
- `GET /api/agent/health`
- `POST /api/agent/wallet-score`
- `POST /api/agent/risk-summary`
- `POST /api/agent/metadata-template`
- `POST /api/agent/chain-health`

## Account and KOSCH access

- `GET|POST /api/account/api-keys`
- `POST /api/account/api-keys/{id}/revoke`

Customer premium routes use authenticated KOSCH entitlement checks. Legacy plan, credit-pack, Shopier, Paddle and local-password routes are not part of production.

## Owner

- `POST /api/owner/login`
- `POST /api/owner/logout`
- `GET /api/owner/command-center`
- `GET /api/owner/operations`
- `GET /api/owner/arvis`
- `POST /api/owner/arvis/scan`
- `GET /api/owner/creator-intelligence`
- `/api/owner/radar/sources`
- `GET /api/owner/kosch-access`
- `GET /api/owner/security-events`
- `GET /api/owner/route-map`
- `/api/owner/feedback`
- `GET /api/owner/users`
- `POST /api/owner/users/ban`
- `POST /api/owner/users/remove`
- `POST /api/owner/command`
- `POST /api/owner/brain`
- `/api/owner/chat`
- `GET /api/owner/health`
- `GET /api/owner/status`
- `/owner`
- `/owner.html`

## Product Radar

Free core:

- `POST /api/token/scan`

KOSCH premium:

- `POST /api/v1/token/extensions`
- `POST /api/v1/address-poisoning/check`
- `GET /api/v1/risk/badge`
- `GET /api/v1/radar/feed`
- `POST /api/v1/radar/check`
- `GET /api/v1/radar/detail`
- `GET /api/v1/radar/creator-intelligence`
- `GET /api/v1/radar/graph`
- `GET /api/v1/radar/exposure`

## Developer API

These routes use API-key authentication, KOSCH access and API rate limits.

- `POST /api/v1/scan/token`
- `GET /api/v1/usage`
- `POST /api/v1/shield/preflight`
- `POST /api/v1/shield/transaction`
- `POST /api/v1/shield/address-poisoning`

## Static serving

Static files are served from `koschei/api/public`. Unknown non-API paths fall back to `index.html`; unknown `/api/*` paths return not found.

## Hygiene rules

- Do not keep disconnected production handlers "for later"; Git history is the archive.
- Do not call a route live unless it is registered here and in `server.go`.
- Customer routes use session authentication; partner routes use API-key authentication.
- Premium routes check KOSCH entitlement.
- Evidence-backed verdicts are signed only when verified evidence exists.
- Applied database migrations remain immutable even when their former feature code is removed.
