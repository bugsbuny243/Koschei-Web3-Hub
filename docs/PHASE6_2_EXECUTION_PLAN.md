# Phase 6.2 Execution Plan — Hızlı İlerleme

Bu doküman, Koschei programını **Phase 6.1'den başlayıp Phase 11'e kadar** kontrollü şekilde taşımak için hızlı ilerleme (execution-first) planını tanımlar.

## Program Strategy

Seçilen strateji: **Hızlı İlerleme (Phase 6.2 first)**

Öncelik sırası:
1. **Phase 6.2** — Async artifact worker migration ve stabilizasyon
2. **Phase 7** — Owner God Mode full lifecycle
3. **Phase 8** — Public SaaS monetization ve abuse hardening
4. **Phase 9** — Media Factory enablement
5. **Phase 10** — Premium UI Universe
6. **Phase 11** — Full ops hardening and scale

---

## Phase 6.2 (Current Active Track)

### Objective
Artifact generation akışını tam async, izlenebilir, retry-safe ve audit-friendly hale getirmek.

### Scope
- Synchronous artifact üretimi yerine worker/queue tabanlı asenkron akış
- Idempotent artifact generation (aynı talepte duplicate üretimi engelleme)
- Retry policy + failure classification
- Download paketleme yolunda güvenlik/erişim doğrulamaları
- Runtime + artifact maliyet/latency telemetrisi

### Deliverables
- Async artifact worker flow (canonical)
- Artifact state machine (`queued`, `processing`, `completed`, `failed`, `canceled`)
- Failure reason taxonomy (provider, validation, parsing, storage, auth)
- Protected artifact download contract (token/auth checked)
- Operator dashboard metrics:
  - generation duration p50/p95
  - failure rate
  - retry success rate
  - per-project artifact count

### Definition of Done (DoD)
- Yeni artifact istekleri sync path kullanmadan async olarak tamamlanır.
- Aynı input ile eşzamanlı isteklerde duplicate artifact üretilmez.
- Tüm final durumlar (`completed/failed/canceled`) audit log'a düşer.
- Başarısız task'larda retry güvenle çalışır ve durum geçmişi korunur.
- API tarafında artifact statüleri tutarlı döner.

### Risks & Mitigation
- **Race condition**: transactional lock + unique idempotency key
- **Queue overload**: backpressure + bounded concurrency
- **Retry storm**: exponential backoff + max retry cap
- **Credit leakage**: charge-on-success veya reversible ledger entry

---

## Phase 7 Plan (Owner God Mode Full)

### Objectives
- Manual fulfillment cockpit
- Revision-aware lifecycle
- QC gate before delivery
- Full audit trail

### Core Work Items
- Order intake -> generation -> revision -> final delivery state transitions
- Owner-only permissions hardening
- Revision diff/history visibility
- Delivery checklist enforcement (quality gate)

---

## Phase 8 Plan (Public SaaS)

### Objectives
- Monetization lifecycle hardening
- Abuse-resistant self-serve user flow

### Core Work Items
- Plan/credit model finalization
- Subscription lifecycle automation roadmap
- Usage-limit enforcement and anti-abuse controls
- Onboarding and first-generation conversion funnel

---

## Phase 9 Plan (Media Factory)

### Objectives
- Multimodal media generation pipeline activation

### Core Work Items
- Image/Video/Audio modules queue orchestration
- Cloudinary artifact catalog standardization
- Media-specific retries and failure analytics

---

## Phase 10 Plan (Premium UI Universe)

### Objectives
- Production dashboard UX evolution

### Core Work Items
- `/ui-lab` production merge strategy
- Module-level immersive navigation rollout
- Runtime Factory command-center UX hardening

---

## Phase 11 Plan (Ops / Hardening)

### Objectives
- Scale-ready, observable and secure production operations

### Core Work Items
- Service scaling policies
- Centralized structured logging
- Incident hooks + alerting expansion
- Cost telemetry + model-level usage reporting
- Production-grade rate limits

---

## 6-Week Execution Cadence (Suggested)

### Week 1-2
- Phase 6.2 async worker migration
- State machine + idempotency + retries

### Week 3
- Artifact observability and audit completion
- Operator metrics and runbook v1

### Week 4
- Phase 7 bootstrap (Owner lifecycle states + QC gate)

### Week 5
- Phase 8 bootstrap (plan lifecycle + usage controls)

### Week 6
- Phase 11 baseline pass (logs, alerts, cost telemetry starter)

---

## Immediate Next Tasks (Actionable)

1. Runtime artifact endpointlerinde sync path tespiti ve async migration checklist'i çıkar.
2. Artifact job state machine için tekil enum + transition guard'larını sabitle.
3. Idempotency key stratejisini API seviyesinde zorunlu hale getir.
4. Retry policy'yi hata tipine göre sınıflandır (transient vs terminal).
5. Artifact ve runtime için ortak telemetry event sözlüğü tanımla.

Bu doküman tamamlandıktan sonra bir sonraki adım teknik görevlerin issue/task formatına bölünmesidir.
