# Koschei Implementation Plan (Go + Python + React Native)

## Objective
Refactor Koschei into a production SaaS + owner-operated delivery system with:
- Go API backend
- Python AI/media workers
- React Native + TypeScript client (mobile + web)
- PostgreSQL/Neon, Cloudinary, Together AI, Railway

No legacy Next.js architecture or deprecated sync-runtime behavior should remain in target state.

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
4. Build Owner God Mode UX baseline:
   - private owner access
   - order intake (manual)
   - generation orchestration
   - delivery packaging view

Deliverables:
- one cross-platform app for mobile + web
- separated public and owner experiences

---

## Phase 4 — Operations & Deployment Baseline

1. Railway service split:
   - go-api
   - python-workers
   - frontend-web
2. Configure Neon + Cloudinary + Together AI secrets
3. Add monitoring/logging bootstrap + basic alerting
4. Add operational playbooks for owner workflows

Deliverables:
- deployable multi-service stack on Railway
- environment templates and runbooks

---

## Phase 5 — Hardening Core

1. Add end-to-end tests across public flow and owner flow
2. Add abuse controls and request throttling baseline
3. Add billing reconciliation checks
4. Security review (sessions, role boundaries, secrets)

Deliverables:
- production-readiness baseline checklist
- launch criteria for internal beta

---

## Phase 5.3 — Runtime Stabilizer

1. Remove legacy sync runtime remnants
2. Harden contract prompt schema
3. Fix file-plan JSON tags
4. Strengthen guardrail validation
5. Make dashboard contract output clearer and explicit

Deliverables:
- canonical async runtime behavior only
- deterministic contract payloads
- strict validation + clearer operator visibility

---

## Phase 6 — Artifact / Code Package Generation

1. Generate real file contents from `file_plan`
2. Add `generated_files` table
3. Produce artifact package output
4. Expose downloadable delivery package for user/owner
5. Keep tool calls strictly controlled and auditable

Deliverables:
- persisted generated-file records
- reproducible artifact package pipeline
- download-ready delivery bundle from dashboard

---

## Phase 7 — Owner God Mode (Full)

1. Fiverr/manual order-fulfillment workspace
2. End-to-end flow: customer prompt → project → delivery file
3. Quality-control stage before release
4. Revision system for iterative delivery
5. Full audit log for owner operations

Deliverables:
- owner fulfillment cockpit
- revision-aware lifecycle
- compliance-friendly operational traceability

---

## Phase 8 — Public SaaS

1. Pricing model definition
2. Plan tiers
3. Credits economy
4. Subscription lifecycle
5. Public user onboarding + generation flow
6. Usage limits and abuse control hardening

Deliverables:
- public-facing monetization system
- self-serve lifecycle from signup to usage
- limit-enforced and abuse-resistant platform behavior

---

## Phase 9 — Media Factory

1. Image Forge module
2. Video Lab module
3. Audio Core module
4. Cloudinary-backed media storage and cataloging
5. Render queue orchestration

Deliverables:
- multimodal media production pipeline
- queue-driven rendering across media types
- unified media artifact management

---

## Phase 10 — Premium UI Universe

1. Migrate `/ui-lab` into the real production dashboard
2. Represent each module as a distinct immersive “3D room” UX space
3. Turn Runtime Factory into a production hangar experience

Deliverables:
- premium dashboard experience integrated with live data
- module-level immersive navigation
- operational Runtime Factory command center UX

---

## Phase 11 — Deployment / Ops / Hardening

1. Railway service separation and scaling rules
2. Monitoring stack expansion
3. Centralized logs
4. Error tracking and incident response hooks
5. Cost telemetry
6. Model-level usage reporting
7. Production rate limits

Deliverables:
- hardened multi-service operations
- cost/quality observability by model and workflow
- incident-ready production posture

---

## Final Phase — Hybrid Security Shield

1. Smart-glasses-assisted operator interface (human-in-the-loop)
2. Integrated physical + cyber security workflow
3. Bank/government-grade security audit readiness
4. Multi-model security team roles:
   - Qwen sentinel
   - DeepSeek detective
   - executive manager model
   - Llama field operator
5. Human-approved defense and audit actions only

Deliverables:
- hybrid security governance layer
- auditable, human-approved defensive operations
- enterprise/public-sector security posture target

---

## Non-Goals / Explicit Exclusions

- Unattended Fiverr account automation
- Fiverr scraping
- bot-driven Fiverr messaging
- fully autonomous external delivery actions without owner approval

All external marketplace delivery remains human-approved and owner-controlled.
