# Koschei Web3 Hub - Solana Ekosistemi İçin Proaktif Güvenlik Katmanı

> **Non-custodial, AI destekli ve public-good güvenlik platformu:** Koschei Web3 Hub; Solana kullanıcıları, geliştiricileri ve DAO ekipleri için MEV, likidite drenajı, governance saldırıları ve riskli akıllı para hareketlerini işlem gerçekleşmeden önce görünür hale getirir.

![Live Demo](https://img.shields.io/badge/Live%20Demo-Online-00ffaa?style=for-the-badge)
![Grant Status](https://img.shields.io/badge/Solana%20Grant-Tier--1%20Ready-7c5cff?style=for-the-badge)
![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)
![Build Status](https://img.shields.io/badge/Build-Go%20API%20%2B%20Vanilla%20JS-success?style=for-the-badge)

---

## Neden Koschei?

Web3 güvenliği çoğu zaman **reaktif** çalışır: exploit olur, zarar ölçülür, rapor yazılır. Koschei bu yaklaşımı tersine çevirir.

| Reaktif Güvenlik | Koschei Proaktif Katmanı |
| --- | --- |
| Saldırıdan sonra forensics | İşlemden önce risk skoru |
| Manuel alarm takibi | Canlı MEV ve likidite radar sinyalleri |
| Kapalı veri siloları | Public-good impact dashboard |
| Custodial entegrasyon riski | **No private keys, no seed phrases, no custody** |

Koschei, kullanıcı fonlarına dokunmadan read-only RPC, veritabanı agregasyonları, risk kuralları ve AI destekli simülasyonlarla Solana ekosisteminde ölçülebilir güvenlik etkisi üretir.

---

## Canlı Metrikler Tablosu

> İlk grant sunumlarında mock başlangıç değerleri kullanılır; production ortamında `/api/public/metrics` endpoint'i `mev_protection_events` ve `liquidity_drain_alerts` tablolarından gerçek agregasyon döndürür.

| Metrik | Başlangıç Değeri | Kaynak |
| --- | ---: | --- |
| Toplam Kurtarılan USD | $128,400+ | MEV Shield + Liquidity Radar |
| Engellenen Saldırılar | 324+ | Koruma olayları ve alarm kayıtları |
| Aktif Cüzdanlar | 1,842+ | Read-only kullanıcı / cüzdan agregasyonu |
| Canlı Modüller | 5 | Public security modules |
| Güncelleme Sıklığı | 60 sn | API cache TTL |

---

## Modüller

### 1. MEV Shield
Swap öncesi sandwich, backrun, slippage ve rota manipülasyonu risklerini analiz eder. Kullanıcıya risk skoru, sebep listesi ve daha güvenli işlem önerisi verir.

### 2. Liquidity Radar
Token havuzlarında ani likidite çıkışı, rug-pull paterni, yüksek riskli pool davranışı ve kritik anomali sinyallerini canlı alarm akışı olarak sunar.

### 3. AI Simulator
Geliştiriciler ve DAO ekipleri için saldırı senaryolarını doğal dil ile simüle eder; riskli kod, konfigürasyon veya operasyon kararı için düzeltme önerileri üretir.

### 4. DAO Guardian
DAO tekliflerinde treasury outflow, yetki devri, parametre manipülasyonu ve governance takeover ihtimallerini işaretler.

### 5. Smart Money Oracle
Whale, fon, bot ve yüksek etki cüzdan hareketlerinden risk sinyalleri çıkarır; güvenlik kararlarına bağlam ekler.

---

## Teknik Mimari

- **Backend:** Go 1.23, net/http tabanlı API, güvenli middleware zinciri
- **Database:** Neon Postgres / PostgreSQL read-write ve read-replica desteği
- **Cache:** Redis veya in-memory fallback
- **Solana Data:** Solana RPC, token / wallet / transaction analiz servisleri
- **AI Layer:** Together AI entegrasyonuna hazır risk simülasyon katmanı
- **Frontend:** Vanilla HTML, CSS ve JavaScript; React/Vue runtime bağımlılığı yok
- **Security:** Non-custodial tasarım, CSP header, owner-only operasyon paneli, API key destekli B2B endpoint'ler

---

## Proje Yapısı

```text
.
├── README.md
├── Dockerfile
├── render.yaml
├── db/
│   └── README.md
└── koschei/
    └── api/
        ├── main.go
        ├── go.mod
        ├── internal/
        │   ├── handlers/
        │   │   ├── impact.go
        │   │   ├── mev_shield.go
        │   │   ├── liquidity_radar.go
        │   │   └── owner.go
        │   ├── http/
        │   │   └── server.go
        │   ├── cache/
        │   ├── db/
        │   └── web3/
        └── public/
            ├── index.html
            ├── impact.html
            ├── mev-shield.html
            ├── radar.html
            └── owner.html
```

---

## Kurulum

### 1. Repoyu klonla

```bash
git clone https://github.com/your-org/Koschei-Web3-Hub.git
cd Koschei-Web3-Hub/koschei/api
```

### 2. Ortam değişkenlerini hazırla

```bash
cp ../../.env.example .env
```

Minimum önerilen değişkenler:

```env
PORT=8080
DATABASE_URL=postgres://user:pass@host:5432/koschei?sslmode=require
DATABASE_READ_URL=postgres://user:pass@replica:5432/koschei?sslmode=require
REDIS_URL=redis://localhost:6379
OWNER_WALLET=your_owner_wallet
OWNER_SECRET=strong_owner_secret
ADMIN_PASSWORD=strong_admin_password
SOLANA_RPC_URL=https://api.mainnet-beta.solana.com
TOGETHER_API_KEY=optional_for_ai_modules
```

### 3. Lokal çalıştır

```bash
go run main.go
```

Ardından:

- Ana sayfa: `http://localhost:8080/`
- Impact Dashboard: `http://localhost:8080/impact`
- MEV Shield: `http://localhost:8080/mev-shield`
- Liquidity Radar: `http://localhost:8080/radar`
- Public Metrics API: `http://localhost:8080/api/public/metrics`

### 4. Test ve build

```bash
go test ./...
go build ./...
```

---

## Owner Panel Notu

Owner Panel gizli operasyon alanıdır. `/owner` route'u owner authentication kontrolünden geçer; `/api/owner/*` operasyon endpoint'leri owner-only middleware ve handler seviyesinde doğrulama ile korunur.

Giriş için gerekli değerler:

- `OWNER_WALLET` veya `KOSCHEI_OWNER_WALLET`
- `OWNER_SECRET` veya `KOSCHEI_OWNER_SECRET`

Panel tarayıcıda güvenli cookie oluşturmak için `/api/owner/login` endpoint'ini kullanır. Production ortamında `APP_ENV=production` ile cookie'ler `Secure` ve `SameSite=Strict` çalışır.

---

## Grant Readiness

Koschei Web3 Hub, Solana Foundation Tier-1 Grant değerlendirmesine uygun şekilde şu kanıtları sunar:

1. **Canlı ürün:** modern responsive arayüz, public dashboard ve çalışan API endpoint'leri.
2. **Ölçülebilir impact:** kurtarılan USD, engellenen saldırılar, aktif cüzdanlar ve son olay logları.
3. **Public-good yaklaşımı:** read-only, non-custodial ve topluluk güvenliğini önceleyen mimari.
4. **Teknik sürdürülebilirlik:** Go backend, Neon Postgres, Redis cache ve vanilla frontend ile düşük operasyonel karmaşıklık.
5. **Modüler büyüme:** MEV Shield, Liquidity Radar, AI Simulator, DAO Guardian ve Smart Money Oracle aynı hub içinde ölçeklenebilir.

---

## Lisans & İletişim

- **Lisans:** MIT
- **Topluluk çağrısı:** Solana geliştiricileri, validator ekipleri, DAO güvenlik ekipleri ve public-good fonlayıcıları iş birliğine davetlidir.
- **İletişim:** `hello@koschei.ai`

---

Built with ❤️ for Solana Ecosystem
