# Koschei — The Immortal AI Platform

Koschei is being rebuilt as a **two-surface AI SaaS platform**:

1. **Public Koschei SaaS** for normal users
2. **Private Owner God Mode** for Onur to manually fulfill Fiverr/client orders

This repository now documents and organizes the platform around a **Go + Python + React Native + TypeScript** architecture.

---

## Core Product Vision

Koschei provides production-grade AI creation workflows for:
- code generation (apps/sites/scripts)
- image generation
- video generation
- audio tooling

Public users access these capabilities through subscription and credit plans.

Owner God Mode is intentionally manual for client-delivery operations:
- no Fiverr login integration
- no scraping
- no bot messaging
- no automatic Fiverr delivery
- owner manually enters order requirements
- Koschei generates outputs
- owner manually delivers work in Fiverr

---

## Mandatory Stack (No Next.js)

> **Important:** Next.js is not part of the target architecture.

- **Backend API:** Go / Golang
- **AI & media workers:** Python
- **Frontend app:** React Native + TypeScript
- **Web delivery of app UI:** React Native Web / Expo Web
- **Database:** PostgreSQL / Neon
- **Media storage:** Cloudinary
- **AI model provider:** Together AI
- **Deployment:** Railway

---

## Target Architecture

### 1) Go Backend Service
Responsible for:
- REST API endpoints
- authentication and session logic
- subscription and credit accounting
- owner role protection
- public SaaS APIs
- private owner order APIs
- database access

### 2) Python Worker Service
Responsible for:
- AI generation jobs
- image/video/audio processing pipelines
- prompt routing
- Cloudinary upload helpers
- background queue execution

### 3) React Native + TypeScript Frontend
Responsible for:
- public landing experience
- authenticated user dashboard
- owner god mode panel
- mobile-ready UI
- web compatibility via React Native Web / Expo Web

### 4) Two Product Surfaces
- **Public SaaS Surface:** user-facing generation and account workflows
- **Owner Surface (God Mode):** private operational workspace for manual client fulfillment

---

## Repository Structure (Refactor Target)

```text
koschei/
  backend/                 # Go REST API service
    cmd/server/
    internal/
      auth/
      handlers/
      services/
      db/
      models/
      billing/
      owner/
  workers/                 # Python workers for generation + media pipelines
    app/
      jobs/
      processors/
      prompt_router/
      cloudinary/
      providers/together/
    tests/
  frontend/                # React Native + TypeScript app
    src/
      features/public/
      features/dashboard/
      features/owner/
      shared/
    app.config.ts
  infra/
    railway/
    migrations/
  docs/
    IMPLEMENTATION_PLAN.md
```

---

## Migration Notes

Current repository contents may include legacy web artifacts from prior experiments. The refactor direction is:

1. Remove framework assumptions tied to Next.js and Next API routes
2. Move API responsibilities into Go service routes
3. Move generation/media logic into Python workers
4. Consolidate UI into React Native + TypeScript with web support through React Native Web / Expo Web

Detailed rollout is in `docs/IMPLEMENTATION_PLAN.md`.

---

## Deployment Targets

- **API:** Railway service for Go backend
- **Workers:** Railway worker service(s) for Python job processing
- **Frontend Web:** Expo Web build deployment on Railway (or static edge target)
- **Database:** Neon PostgreSQL
- **Media:** Cloudinary assets and delivery

---

## Status

This repo is in active architecture transition toward the stack above.
