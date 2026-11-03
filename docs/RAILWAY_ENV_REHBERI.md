# Railway env rehberi — çekirdek, güvenlik merkezi (SOC) ve çoklu ağ taraması

Bu rehber `koschei/api` kodunun **gerçekten okuduğu** env adlarından çıkarıldı
(2026-11-03, `main` @ `ce0248e`). Değer içermez; secret'lar yalnızca Railway'in
korumalı değişken editörüne girilir, sohbete, koda veya loga yazılmaz.

İşaretler: **🔐** secret (sağlayıcı paneli veya `openssl rand -base64 48` ile üretilir) ·
**⚠️ fail-closed** (yanlış/eksikse servis açılışta `log.Fatalf` ile durur) ·
**🟢** okuma anında kullanılır, açılışı etkilemez.

> Tek servis (`KOSCHEI_RUNTIME_ROLE` boş = `combined`) çalıştığı için fail-closed bir
> bayrak yanlış ayarlanırsa **API de düşer**. Aşama 2'ye geçmeden önce Railway'de
> healthcheck yolunu `/health` yapın: açılışta çöken deploy başarısız sayılır ve eski
> deploy çalışmaya devam eder.

---

## 1. Çekirdek — production açılışı için zorunlu

`APP_ENV=production` iken aşağıdakilerden biri boşsa servis açılmaz (⚠️ fail-closed).

| Env | Not |
|---|---|
| `APP_ENV=production` | Production guard'ı ve canonical job worker varsayılanını açar. |
| `API_KEY_PEPPER` 🔐 | Müşteri API anahtarı hash pepper'ı. Değiştirmek mevcut anahtarları geçersiz kılar. |
| `USER_SESSION_SECRET` 🔐 | Oturum imzası. |
| `OWNER_SECRET` 🔐 | Owner erişimi. |
| `NEON_AUTH_JWKS_URL` | Neon Auth JWT doğrulaması. |
| `KOSCHEI_VERDICT_SIGNING_KEY_ID` / `KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY` 🔐 | İmzalı nihai karar (Ed25519). |
| `DATABASE_URL` 🔐 | Guard zorunlu tutar (`KOSCHEI_NEON_AUTH_ONLY=true` ise hariç). TradePI agents, defense worker/sentinel ve `cmd/` araçları da okur. |

Zorunlu değil ama ürün için gerekli:

| Env | Not |
|---|---|
| `APP_DATABASE_URL` 🔐 | API kalıcılığı (hesaplar, watchlist, job, webhook, migration). Boşsa API **stateless** çalışır, `DATABASE_URL`'e düşmez. |
| `APP_DATABASE_READ_URL` 🔐 | Opsiyonel read replica. |
| `ENTITLEMENT_DATABASE_URL` 🔐 | Ücretli erişim defteri; boşsa ücretli işlemler fail-closed kalır. |
| `WEBHOOK_ENCRYPTION_KEY` 🔐 | Kayıtlı webhook secret'larını şifreler. **Bir kez ayarlandıktan sonra değiştirmeyin.** |
| `NEON_AUTH_ISSUER`, `NEON_AUTH_BASE_URL`, `EXPO_PUBLIC_NEON_AUTH_URL` | Neon Auth. |
| `PUBLIC_APP_URL`, `CORS_ORIGIN` | `CORS_ALLOWED_ORIGIN` eski isimdir, yedek olarak hâlâ okunur. |

---

## 2. Siber güvenlik merkezi (SOC) env'leri

### 2.1 Kimlik, oturum, istemci IP'si

| Env | Varsayılan / davranış |
|---|---|
| `NEON_AUTH_STATE_SECRET` 🔐 | Boşsa `USER_SESSION_SECRET` kullanılır. Bağımsız bir değer önerilir. |
| `NEON_AUTH_AUDIENCE` | Boşsa JWT `aud` kontrol edilmez (issuer kontrol edilir). Yanlış değer tüm girişleri reddeder. |
| `TRUSTED_PROXY_CIDRS` | Boşsa doğrudan bağlanan adres kullanılır. Railway proxy'si arkasında herkes aynı IP olarak görünür: rate limit tek ortak sayaç olur, audit loglarda gerçek IP olmaz. **Railway'in proxy aralığını doğrulamadan girmeyin**, geniş girilen aralık `X-Forwarded-For` sahteciliğine izin verir. |

### 2.2 Alarm kanalları

Yalnızca yapılandırılmış kanallara kuyruk açılır; hiçbiri yoksa alarmlar uygulama içinde kalır.

| Env | Not |
|---|---|
| `DISCORD_WEBHOOK_URL` 🔐 | Yalnızca resmi `https://discord.com/api/webhooks/...` kabul edilir. **SOC için önerilen kanal.** |
| `TELEGRAM_BOT_TOKEN` 🔐 + `TELEGRAM_CHAT_ID` | Dikkat: aynı token TradePI agents Telegram gönderimi tarafından da okunur. Ayrı tutmak istiyorsanız SOC için Discord kullanın. |
| `SECURITY_ALERT_MIN_SEVERITY` | Varsayılan `high`. |
| `WHITEHAT_ALERT_ADDRESSES` | Opsiyonel. |

### 2.3 Transaction Guard / firewall

| Env | Not |
|---|---|
| `KOSCHEI_TRANSACTION_FIREWALL_ENABLED` | Varsayılan `true`. |
| `TRANSACTION_GUARD_ENFORCEMENT_KEY_ID` / `_PRIVATE_KEY` 🔐 | İmzalı enforcement permit. |
| `TRANSACTION_GUARD_ENFORCEMENT_PERMIT_TTL_SECONDS` | 10–600, varsayılan 90. |
| `TRANSACTION_GUARD_REQUIRE_ENFORCEMENT_PERMIT` | `true` = permit alınamazsa ALLOW kararı da bloklanır. |
| `TRANSACTION_GUARD_REQUIRE_STATE_WITNESS`, `_REQUIRE_SIGNED_INTENT`, `_REQUIRE_CPI_FLOW`, `_REQUIRE_AUTHORITY_SURFACE`, `_REQUIRE_THREAT_HISTORY` | Ek fail-closed kapılar; hepsi varsayılan kapalı. Tek tek ve test ederek açın. |
| `TRANSACTION_GUARD_BLOCKED_PROGRAMS` | Operatör denylist'i. |

### 2.4 Zincir üstü izleme ve SOC iş akışı

| Env | Not |
|---|---|
| `KOSCHEI_AUTOMATIC_SCANNING_ENABLED` | Arka plan taramalarının ana anahtarı (watchlist monitor bunu da ister). |
| `WATCHLIST_MONITOR_ENABLED`, `WATCHLIST_MONITOR_INTERVAL`, `WATCHLIST_MONITOR_BATCH_SIZE` | Watchlist izleme. |
| `KOSCHEI_SOLANA_AUTHORITY_WATCH_ENABLED` / `_ADDRESSES` (1–3 hesap) / `_INTERVAL_SECONDS` (60–900) | SOC PB02 sınırlı authority izleme. |
| `KOSCHEI_SOC_REVIEW_POLICY_JSON` | Opsiyonel inceleme politikası. |
| `ARVIS_STREAM_CANONICAL_JOBS_ENABLED` + `KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON` | Stream tabanlı imzalı final kararlar; ayrıca `SOLANA_RPC_URL` gerekir. |

### 2.5 Kanıt saklama ve arşiv

| Env | Not |
|---|---|
| `KOSCHEI_RADAR_ARCHIVE_EXPORT_SINK` | `disabled` \| `filesystem` \| `s3`. |
| `KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT` | **Şema dahil tam URL** (`https://...`). Şemasız değer retention worker'ını durdurur. |
| `KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_BUCKET`, `_REGION`, `_ACCESS_KEY_ID` 🔐, `_SECRET_ACCESS_KEY` 🔐, `_PATH_STYLE` | S3/R2/MinIO. |
| `GOOGLE_DRIVE_ARCHIVE_FOLDER_ID` + `GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON` 🔐 | İkisi birlikte yoksa Drive arşivi devre dışıdır. |

### 2.6 Defense OS (Solana)

Web servisi: `KOSCHEI_DEFENSE_AGENT_RUNTIME_ENABLED`, `KOSCHEI_DEFENSE_KNOWLEDGE_EMBEDDINGS_ENABLED`,
`KOSCHEI_DEFENSE_SOURCE_IMPORT_ENABLED`, `KOSCHEI_DEFENSE_HARNESS_PLANNER_ENABLED`,
`KOSCHEI_DEFENSE_HARNESS_EXECUTION_GATE_ENABLED`, `KOSCHEI_DEFENSE_HARNESS_MATERIALIZATION_ENABLED`,
`KOSCHEI_DEFENSE_REPRODUCTION_ENABLED`, `KOSCHEI_DEFENSE_WORKER_QUEUE_ENABLED`,
`KOSCHEI_DEFENSE_SENTINEL_MANAGEMENT_ENABLED`, `KOSCHEI_DEFENSE_PATCH_PROPOSAL_ENABLED`.
Ayrı worker / sentinel servisleri için `docs/defense-*.env.example` dosyalarına bakın.

---

## 3. Çoklu ağ taraması

Üç ayrı yol var; her biri farklı env okur:

1. **Anlık müşteri taraması / EVM probe / native job**: zincire özel `*_RPC_URL`.
2. **Radar adapter kaydı** (kontrat, token, governance, Tron/TON/Cosmos/Stellar/offchain): tek bir `RADAR_PROVIDERS_JSON` 🔐.
3. **Sürekli küresel radar** (ağ sağlığı + blok ingest): ClickHouse ve bölüm 4.

> Radar adapter istekleri Solana ile **aynı saatlik RPC bütçesini** paylaşır
> (`SOLANA_RPC_BUDGET_MAX_REQUESTS`, `SOLANA_RPC_BUDGET_WINDOW_SECONDS`). Ağ ekledikçe
> bütçeyi sağlayıcı kotanıza göre artırın.

### 3.1 Ağ tablosu

| Ağ | Anlık tarama env'i | Radar adapter (`RADAR_PROVIDERS_JSON` anahtarı) | Sürekli radar |
|---|---|---|---|
| Solana | `SOLANA_RPC_URL`, `HELIUS_SOLANA_RPC_URL`, `ALCHEMY_SOLANA_RPC_URL`, `QUICKNODE_SOLANA_RPC_URL`, `SOLANA_RPC_FALLBACK_URL`, `HELIUS_API_KEY` 🔐, `ALCHEMY_API_KEY` 🔐; stream: `SOLANA_WSS_URL` (+ sağlayıcı WSS'leri) | `solana-mainnet` | Mevcut Solana radar/stream worker'ları |
| Ethereum | `ETHEREUM_RPC_URL` 🔐 | `ethereum-mainnet` | ✓ (+ `ETHEREUM_BEACON_URL`) |
| Base | `BASE_RPC_URL` 🔐 | `base-mainnet` | ✓ |
| Arbitrum | `ARBITRUM_RPC_URL` 🔐 | `arbitrum-mainnet` | ✓ |
| Optimism | `OPTIMISM_RPC_URL` 🔐 | `optimism-mainnet` | ✓ |
| Polygon | `POLYGON_RPC_URL` 🔐 | `polygon-mainnet` | ✓ |
| BNB Chain | `BNB_RPC_URL` 🔐 | `bnb-mainnet` | ✓ |
| Avalanche C | `AVALANCHE_RPC_URL` 🔐 | `avalanche-mainnet` | ✓ |
| Bitcoin | `BITCOIN_ESPLORA_URL` (adres/işlem) | — (ayrı route) | `BITCOIN_CORE_RPC_URL` 🔐 |
| Sui | `SUI_GRAPHQL_URL` | — (ayrı route) | — |
| Aptos | `APTOS_REST_URL` | — (ayrı route) | — |
| Tron | `TRONGRID_API_KEY` 🔐 (wallet permission collector) | `tron-mainnet`: `primary`+`secondary` (`/wallet`, `/walletsolidity`) + `indexed` (TronGrid V1), header `TRON-PRO-API-KEY` | — |
| TON | — | `ton-mainnet`: toncenter uyumlu `/api/v2` **ve** `/api/v3`, header `X-API-Key` (TonAPI uyumlu değil). JSON'da `ton-mainnet` yoksa `TONCENTER_API_KEY` 🔐 tek başına yeterli: primary otomatik olarak `https://toncenter.com` olur. | — |
| Linea, Scroll, zkSync, Berachain, HyperEVM, Monad, Sei EVM | — | `linea-mainnet`, `scroll-mainnet`, `zksync-mainnet`, `berachain-mainnet`, `hyperevm-mainnet`, `monad-mainnet`, `sei-evm-mainnet` (EIP-1898 destekli JSON-RPC) | — |
| Cosmos Hub, Osmosis, Celestia, Injective, Sei | — | `cosmoshub-mainnet`, `osmosis-mainnet`, `celestia-mainnet`, `injective-mainnet`, `sei-mainnet` (LCD REST) | — |
| Stellar | — | `stellar-mainnet` (Stellar JSON-RPC) | — |
| Offchain (dApp frontend) | — | `offchain:<host>` + `RADAR_FRONTEND_BASELINES_JSON` | — |

**Env ile açılamayan ağlar** (kodda collector yok, `declared_not_executable`):
Bitcoin Cash, Cardano, Dogecoin, Litecoin, NEAR, Polkadot, Starknet, XRPL, Hyperliquid Core.
Bunlar için yeni adapter yazılması gerekir; env eklemek tarama sağlamaz.

### 3.2 `RADAR_PROVIDERS_JSON` kuralları

- URL **HTTPS** olmalı; query string, `user:pass@` ve `#fragment` reddedilir. Anahtarlar header'a konur.
  Yol içindeki anahtarlar (ör. `/v2/<key>`) kabul edilir, sonuçlara ve loglara yazılmaz.
- Rol başına `requests_per_second` ve `daily_limit` sağlayıcı sözleşmenize göre ayarlanır
  (varsayılan 1 rps / 10.000 gün, **süreç başına**).
- Railway'de tek satır JSON olarak girin. Örnek yapı (geçersiz `.invalid` hostlar, gerçek değil):

```json
{"ethereum-mainnet":{"primary":{"url":"https://evm-a.example.invalid/v2/KEY","requests_per_second":2,"daily_limit":20000},"secondary":{"url":"https://evm-b.example.invalid","requests_per_second":1,"daily_limit":10000}},"tron-mainnet":{"primary":{"url":"https://api.trongrid.io","headers":{"TRON-PRO-API-KEY":"KEY"}},"secondary":{"url":"https://tron-b.example.invalid"},"indexed":{"url":"https://api.trongrid.io","headers":{"TRON-PRO-API-KEY":"KEY"}}},"ton-mainnet":{"primary":{"url":"https://toncenter.com","headers":{"X-API-Key":"KEY"}}}}
```

Opsiyonel pin kayıtları: `RADAR_EVM_SWEEPER_HASHES_JSON`, `RADAR_TON_JETTON_MASTERS_JSON`
(jetton_wallet bakiye atfı için gerekli), `RADAR_FRONTEND_BASELINES_JSON`.

Doğrulama: `koschei/api` içinde `go run ./cmd/radar-smoke` (ağ çağrısı yapmadan yapılandırma
durumu) ve owner oturumuyla `GET /api/owner/radar/capabilities`.

---

## 4. Sürekli küresel radar (ClickHouse) — ⚠️ fail-closed

Sıra önemlidir. Her adımdan sonra deploy loglarında `CRITICAL` satırı olmadığını kontrol edin.

1. **ClickHouse bağlantısı**: `CLICKHOUSE_HTTP_URL` (https), `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD` 🔐, `CLICKHOUSE_DATABASE`.
2. **Şema**: GitHub Actions'ta `ClickHouse Schema Apply` / cutover one-shot workflow'larını çalıştırın (graph, event, checkpoint ve gap tabloları).
3. `KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED=1` → graph şeması doğrulanır, yoksa açılış durur.
4. `KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED=1` → event + checkpoint + gap şemaları doğrulanır.
5. Ağ sağlığı telemetrisi: `KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED=1`,
   `KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS=ethereum-mainnet,...` (her ağın `*_RPC_URL`'i dolu olmalı),
   `KOSCHEI_GLOBAL_RADAR_BACKGROUND_INTERVAL_SECONDS` (300–3600).
6. Blok ingest: `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED=1`, `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS=...`,
   `_INTERVAL_SECONDS` (5–300), `_MAX_BLOCKS_PER_CYCLE` (1–64), `_MAX_EVENTS_PER_BLOCK` (1–25000).
   Desteklenen: ethereum, base, arbitrum, optimism, polygon, bnb, avalanche, bitcoin (`-mainnet`).
7. İki sağlayıcılı doğrulama (opsiyonel): `KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION=1`,
   `_AUTO_REORG_RECOVERY=1`, `_MAX_REORG_REWIND` ve seçilen her ağ için
   `KOSCHEI_GLOBAL_RADAR_CONFIRMATION_<AĞ>_RPC_URL` 🔐 (birincil sağlayıcıdan **farklı** bir sağlayıcı).
8. Kampanya motoru: `KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED=1` (`APP_DATABASE_URL`, graph deposu ve migration 129+ gerekir).

Öneri: sürekli radarı ayrı bir Railway servisinde (`KOSCHEI_RUNTIME_ROLE=worker`) çalıştırıp web
servisini `api` yapmak, ingest kaynaklı bir çöküşün müşteri API'sini düşürmesini engeller.

---

## 5. Aşamalı Railway şablonu

Railway → servis → **Variables → Raw Editor**. **Mevcut içeriği silmeyin**; satırları en alta
ekleyin (Raw Editor'ün tamamını değiştirmek mevcut değişkenleri siler). Boş bırakılan
satırları eklemeyin. Değişiklikler uygulandığında servis yeniden deploy olur.

### Aşama 1 — açılışı etkilemez (🟢)

```env
# Alarm (birini veya ikisini)
DISCORD_WEBHOOK_URL=
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
SECURITY_ALERT_MIN_SEVERITY=high

# Kimlik
NEON_AUTH_STATE_SECRET=

# Arşiv düzeltmesi (tam URL)
KOSCHEI_RADAR_ARCHIVE_EXPORT_S3_ENDPOINT=https://

# EVM anlık tarama
BASE_RPC_URL=
ARBITRUM_RPC_URL=
OPTIMISM_RPC_URL=
POLYGON_RPC_URL=
BNB_RPC_URL=
AVALANCHE_RPC_URL=

# Bitcoin / Move
BITCOIN_ESPLORA_URL=
SUI_GRAPHQL_URL=
APTOS_REST_URL=

# Radar adapter kaydı (tek satır JSON)
RADAR_PROVIDERS_JSON=
```

Bu aşamaya bilerek konmayanlar:
- `TRUSTED_PROXY_CIDRS`: yalnızca Railway proxy aralığı doğrulandıktan sonra.
- `NEON_AUTH_AUDIENCE`: yanlış değer **tüm girişleri reddeder**; yalnızca Neon Auth token'larındaki `aud` değeri kesin biliniyorsa.
- `CORS_ORIGIN`: `CORS_ALLOWED_ORIGIN`'den önce okunur. Taşımak isterseniz mevcut `CORS_ALLOWED_ORIGIN` değerini aynen kopyalayın, sonra eskisini silin.

### Aşama 2 — ClickHouse (⚠️ önce healthcheck + şema)

```env
CLICKHOUSE_PASSWORD=
KOSCHEI_GLOBAL_RADAR_CLICKHOUSE_ENABLED=1
KOSCHEI_GLOBAL_RADAR_EVENT_CLICKHOUSE_ENABLED=1
```

### Aşama 3 — sürekli radar (⚠️)

```env
KOSCHEI_GLOBAL_RADAR_BACKGROUND_ENABLED=1
KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS=ethereum-mainnet
KOSCHEI_GLOBAL_RADAR_BACKGROUND_INTERVAL_SECONDS=900
KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED=1
KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_NETWORKS=ethereum-mainnet
KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_INTERVAL_SECONDS=15
ETHEREUM_BEACON_URL=
```

Ağları birer birer ekleyin (`ethereum-mainnet,base-mainnet,...`), her eklemede sağlayıcı kotasını izleyin.

### Aşama 4 — doğrulama ve kampanya (⚠️)

```env
KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION=1
KOSCHEI_GLOBAL_RADAR_CONFIRMATION_ETHEREUM_RPC_URL=
KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED=1
```

---

## 6. Railway Agent / yardımcı için hazır prompt

```text
Railway projesi "exciting-spontaneity", servis "Koschei-Web3-Hub", ortam "production".
Kurallar: secret değerlerini okuma, yazdırma veya loglama. Değeri olmayan değişken
oluşturma. Mevcut değişkenleri silme veya yeniden adlandırma. Her aşamayı ayrı deploy et.

1) Deploy ayarlarında healthcheck path'i "/health" yap.
2) Aşama 1: secret değerleri (RPC URL'leri, token'lar, JSON) Railway editörüne ben
   kendim gireceğim; bunları bana sorma. Ben "girdim" dedikten sonra
   docs/RAILWAY_ENV_REHBERI.md bölüm 5 Aşama 1 listesindeki değişkenlerin tanımlı
   olduğunu yalnızca adlarıyla kontrol et ve eksikleri listele.
3) Deploy sonrası loglarda şunları kontrol et ve sonucu bildir:
   - "CRITICAL" satırı olmamalı
   - "radar retention v2: halted" satırı olmamalı
   - "api listening on :8080" görünmeli
4) Aşama 2'ye yalnızca ben "ClickHouse şeması uygulandı" dedikten sonra geç.
   CLICKHOUSE_PASSWORD girilmeden KOSCHEI_GLOBAL_RADAR_*_CLICKHOUSE_ENABLED=1 yapma.
5) Aşama 3'te KOSCHEI_GLOBAL_RADAR_BACKGROUND_NETWORKS ve HEAD_INGEST_NETWORKS listesine
   yalnızca *_RPC_URL değeri dolu olan ağları ekle; önce sadece ethereum-mainnet ile başla.
6) Herhangi bir deploy başarısız olursa bir önceki başarılı deploy'a rollback yap ve
   hatanın log satırını (secret içermeyen kısmı) bana göster.
```
