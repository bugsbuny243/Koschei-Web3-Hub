# Proje Durum ve Mimari Raporu

## 1. PROJE ÖZETİ VE MEVCUT DURUM ANALİZİ

### Projenin Amacı
Koschei; çoklu ürün yüzeylerini (SaaS komut deneyimi, runtime orchestration, artifact üretimi, owner operasyon paneli ve cyber defense analizleri) tek bir platformda birleştiren bir “AI command universe” olarak konumlanır. Ana hedef; kullanıcıdan gelen komut/prompt akışlarını API katmanında işleyip, runtime görevleri ve üretim çıktıları (jobs/artifacts) olarak yönetmektir.

### Mevcut Aşama
- Kök README’de resmi faz **“Phase 6.1 — Artifact Stabilizer + Cyber Defense Center Phase 1”** olarak belirtilmiştir.
- Kod tabanında hem tamamlanmış modüller (auth, credits, runtime, artifacts, cyber) hem de “geçici/mock” çalışma izleri vardır:
  - `model_router` gerçek inference yerine mock route döndürür.
  - Billing kısmı README’de manuel aktivasyon olarak geçer.
  - Media modüllerinin bir kısmı “paused/optional” olarak not edilmiştir.
- Bu nedenle proje teknik olarak production’a yakın çalışan bir backend/frontend omurgasına sahip olsa da; bazı kritik yüzeylerde (otomatik ödeme, gerçek model inference, ileri owner modları) roadmap kaynaklı eksikler mevcuttur.

### Kullanılan Teknolojiler (Repo’da görülen)
- **Backend:** Go `1.23.0`, `github.com/lib/pq v1.10.9`.
- **Frontend:** Expo `~53.0.13`, React `19.0.0`, React Native `0.79.5`, TypeScript `~5.8.3`, NativeWind `^4.1.23`.
- **Monorepo/Tooling:** Node `20.x`, npm workspaces-benzeri kök script yönlendirmesi, Turbo (`turbo.json`).
- **DB:** PostgreSQL (SQL migrasyonları ve `lib/pq` sürücüsü üzerinden).
- **Worker:** Python `psycopg2-binary==2.9.9`.

---

## 2. PROJE İÇERİĞİ VE ANA MODÜLLER

### Temel Bileşenler
1. **Go API Servisi (`koschei/api`)**
   - HTTP sunucusu, route binding, auth, runtime, jobs, AI ve cyber endpoint’leri.
2. **Frontend (`koschei/frontend`)**
   - Expo Router tabanlı çok sayfalı arayüz (dashboard, pricing, billing, m-* AI ekranları, cyber-defense).
3. **DB Migrasyon Katmanı (`migrations`)**
   - Canonical schema + feature tabanlı ek migrasyonlar (artifact, cyber, auth hesap geçişleri vb.).
4. **Worker (`koschei/workers`)**
   - Python tabanlı yardımcı işleyici iskeleti.

### Modüller Arası Etkileşim (Özet Akış)
- Frontend, `src/lib/api.ts` benzeri istemci kodu üzerinden Go API’ye çağrı yapar.
- Go API route’ları handler katmanına yönlendirir (`internal/http/server.go` → `internal/handlers/*`).
- Handler’lar PostgreSQL tablolarına yazar/okur (`runtime_projects`, `runtime_tasks`, `generation_jobs`, `generated_artifacts`, `cyber_analyses` vb.).
- Runtime/artifact/cyber süreçleri sonuçları DB’ye log/çıktı olarak kaydeder; frontend bu sonuçları API’den listeler.

---

## 3. GITHUB REPO - KLASÖR VE DOSYA YAPISI

### Kökten Kritik Ağaç (node_modules hariç)
```text
.
├── README.md
├── package.json
├── turbo.json
├── Dockerfile
├── railway.toml
├── docs/
│   ├── KOSCHEI_MASTER_ROADMAP.md
│   ├── IMPLEMENTATION_PLAN.md
│   ├── database-inventory.md
│   └── ...
├── migrations/
│   ├── 001_canonical_schema.sql
│   ├── 002_runtime_tables.sql
│   ├── 008_auth_accounts.sql
│   ├── 018_artifact_generation.sql
│   ├── 021_cyber_analyses.sql
│   └── ...
└── koschei/
    ├── api/
    │   ├── main.go
    │   ├── go.mod
    │   └── internal/
    │       ├── db/db.go
    │       ├── http/server.go
    │       ├── router/model_router.go
    │       └── handlers/*.go
    ├── frontend/
    │   ├── app/*.tsx
    │   ├── src/lib/*.ts
    │   ├── src/components/*.tsx
    │   └── package.json
    └── workers/
        ├── worker.py
        └── requirements.txt
```

### Kritik Klasörlerin Rolü
- `koschei/api/internal/http`: Endpoint mapping, middleware benzeri güvenlik/CORS/DB readiness akışı.
- `koschei/api/internal/handlers`: Domain use-case’lerinin toplandığı iş katmanı.
- `koschei/api/internal/db`: DB bağlantı, migrasyon çalıştırma, schema verification, canonical plan seed.
- `migrations`: Gerçek veri modeli kaynağı; tablo/indeks/fk tanımları burada.
- `koschei/frontend/app`: Route bazlı UI ekranları.
- `koschei/frontend/src/lib`: API ve auth entegrasyon yardımcıları.

### Mimari Analiz
- **Backend:** Katmanlı (Layered) + hafif “feature handlers” yaklaşımı.
  - Transport: `internal/http`
  - Use-case/Controller: `internal/handlers`
  - Infra/Data: `internal/db`
- **Frontend:** Expo Router dosya-tabanlı route mimarisi + shared lib/components.
- **Genel:** Tam bir Clean/Hexagonal sınırı katı değil; handler’lar DB’ye yakın çalışıyor. Yine de sorumluluklar klasör seviyesinde ayrılmış.

---

## 4. VERİ TABANI (NEOM / NEO4J) VE ŞEMA YAPISI

### Önemli Tespit
Bu repoda **Neo4j/graph DB şeması bulunmuyor**. Migrasyonlar ve backend sürücüsü açık şekilde **PostgreSQL** gösteriyor. Dolayısıyla Node Label/Relationship tipi tanımı kaynak kodda mevcut değil.

### PostgreSQL Tabloları ve İlişkiler (mevcut gerçek şema)
Aşağıdaki tablolar doğrudan `001_canonical_schema.sql` ve ilgili ek migrasyonlarda tanımlıdır:
- `plans`
- `app_user_profiles`
- `payment_requests`
- `credit_events`
- `generation_jobs`
- `model_route_logs`
- `runtime_projects`
- `runtime_tasks`
- `runtime_logs`
- `owner_client_orders`
- `owner_order_requirements`
- `owner_order_assets`
- `owner_delivery_packages`
- `owner_revision_requests`
- `owner_profit_records`
- `owner_service_templates`
- `generated_artifacts`
- `generated_files`
- `cyber_analyses`
- `schema_migrations` (runtime’da oluşturuluyor)

### Property/Column ve Constraint Özeti
- PK: çoğunlukla `uuid` + `gen_random_uuid()` veya metin id (`plans.id`).
- FK:
  - `runtime_tasks.project_id -> runtime_projects.id`
  - `runtime_logs.project_id -> runtime_projects.id`
  - `runtime_logs.task_id -> runtime_tasks.id`
  - Owner alt tabloları `owner_client_orders.id`’ye bağlı.
  - `generated_files.artifact_id -> generated_artifacts.id`
- İndeks:
  - `app_user_profiles(lower(email))`
  - `generated_artifacts(runtime_project_id)`
  - `generated_files(artifact_id, runtime_project_id)`
  - `cyber_analyses(auth_subject, created_at desc)`
  - `cyber_analyses(user_email, created_at desc)`

### Mantıksal Şema (ASCII)
```text
app_user_profiles --(plan_id semantic ref)--> plans

runtime_projects 1 --- n runtime_tasks
runtime_projects 1 --- n runtime_logs
runtime_tasks    1 --- n runtime_logs (task_id nullable)

runtime_projects 1 --- n generated_artifacts
generated_artifacts 1 --- n generated_files

owner_client_orders 1 --- n owner_order_requirements
owner_client_orders 1 --- n owner_order_assets
owner_client_orders 1 --- n owner_delivery_packages
owner_client_orders 1 --- n owner_revision_requests
owner_client_orders 1 --- n owner_profit_records (nullable relation)

cyber_analyses (standalone analysis records)
generation_jobs, model_route_logs, payment_requests, credit_events (event/log style standalone)
```

---

## 5. KRİTİK DOSYALAR VE KOD ANALİZİ

1. **`koschei/api/internal/http/server.go`**
   - Tüm public/owner/runtime/ai/cyber endpoint’lerinin route binding merkezi.
   - DB readiness gate, CORS, security headers, static frontend serve fallback burada.
   - Sistemin “entry routing contract” dosyası.

2. **`koschei/api/internal/db/db.go`**
   - DB bağlantısı, pool ayarları, ping health.
   - Migrasyonların dosyadan çalıştırılması ve `schema_migrations` tracking.
   - Required table verification + canonical plan upsert.

3. **`koschei/api/main.go`**
   - Servis bootstrap: env okuma, DB init, server init, listen.
   - Deploy/runtime ortamının kritik başlangıç noktası.

4. **`migrations/001_canonical_schema.sql`**
   - İş modelinin ana veri sözlüğü (kullanıcı, plan, ödeme, kredi, üretim, runtime, owner operasyon).
   - Tüm üst seviye domain varlıklarını tanımlar.

5. **`migrations/018_artifact_generation.sql` + `migrations/021_cyber_analyses.sql`**
   - Faz 6.1 odağındaki Artifact ve Cyber alanlarının veri modeli.
   - Yeni feature surface’lerinin DB’deki ana karşılığı.

---

## Sonuç
Bu repo; tek servisli Go API + Expo frontend + PostgreSQL üzerine kurulu, çok-modüllü bir AI orchestration ürünüdür. Mimari üretimde çalışabilecek kadar bütünleşik olsa da; model inference, billing otomasyonu ve bazı ileri faz yeteneklerde bilinçli olarak roadmap’e bırakılmış alanlar vardır. Neo4j/graph veri modeli mevcut değildir; aktif şema tamamen relational PostgreSQL üzerindedir.
