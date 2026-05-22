# Koschei Implementation Plan (Go + Python + React Native)

## Objective
Refactor Koschei into a production SaaS system with:
- Go API backend
- Python AI/media workers
- React Native + TypeScript client (mobile + web)
- PostgreSQL/Neon, Cloudinary, Together AI, Railway

No Next.js architecture should remain in target state.

---

## Phase 0 — Foundation & Contracts

1. Define domain model:
   - users, sessions, subscriptions, credits
   - generation jobs, assets, orders, owner notes
2. Define API contract (OpenAPI):
   - public auth endpoints
   - public generation endpoints
   - subscription/credit endpoints
   - owner-only order endpoints
3. Define queue contract between Go and Python workers.

Deliverables:
- API schema
- database schema draft
- worker job payload schema

---

## Phase 1 — Go Backend Service

1. Build core REST server and middleware
2. Implement auth/session flow
3. Implement credits + subscription accounting
4. Build public APIs:
   - create generation request
   - list/history/status
5. Build owner APIs:
   - create/update client order
   - trigger generation from owner dashboard
   - publish/download packaged output
6. Integrate PostgreSQL (Neon)

Deliverables:
- running Go API with DB migrations
- role-aware authorization
- public + owner endpoint coverage

---

## Phase 2 — Python Worker Service

1. Build job runner + queue consumer
2. Implement Together AI prompt routing
3. Implement image/video/audio processors
4. Implement Cloudinary upload/storage adapter
5. Persist output status/artifacts back through API/database

Deliverables:
- asynchronous generation pipeline
- retry/failure states
- artifact upload and retrieval

---

## Phase 3 — React Native + TypeScript Frontend

1. Establish shared app shell with React Native + Expo
2. Enable React Native Web / Expo Web output
3. Build public SaaS UX:
   - landing
   - sign-in/sign-up
   - user dashboard
   - credits/subscription view
   - generation workflows
4. Build Owner God Mode UX:
   - private owner access
   - order intake (manual)
   - generation orchestration
   - delivery packaging view

Deliverables:
- one cross-platform app for mobile + web
- separated public and owner experiences

---

## Phase 4 — Operations & Deployment

1. Railway service split:
   - go-api
   - python-workers
   - frontend-web
2. Configure Neon + Cloudinary + Together AI secrets
3. Add monitoring/logging + basic alerting
4. Add operational playbooks for owner workflows

Deliverables:
- deployable multi-service stack on Railway
- environment templates and runbooks

---

## Phase 5 — Hardening

1. Add end-to-end tests across public flow and owner flow
2. Add abuse controls and request throttling
3. Add billing reconciliation checks
4. Security review (sessions, role boundaries, secrets)

Deliverables:
- production readiness checklist
- launch criteria for public beta

---

## Non-Goals / Explicit Exclusions

- Fiverr account automation
- Fiverr scraping
- bot-driven Fiverr messaging
- automated Fiverr delivery actions

All Fiverr delivery remains manual by owner action.
