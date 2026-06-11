# Koschei Web3 Hub Investor Tech Deck Taslağı

> Amaç: Koschei Web3 Hub MVP'sini, Solana ekosistemine ölçülebilir güvenlik faydası sağlayan ve DAO/fon/market-maker gibi kurumsal müşterilere satılabilir altı enterprise modülle grant ve Series A seviyesine taşıyacak teknik yatırım tezini tanımlamak.

---

## Sayfa 1 — Yatırım Değerlemesi Tezi ve Stratejik Konumlandırma

### Kısa değerleme analizi

Koschei'nin mevcut MVP'si; cüzdan skoru, token tarama, risk sinyalleri, işlem açıklama ve admin/analitik altyapısı ile “okunabilir Web3 risk istihbaratı” pozisyonundadır. Altı enterprise modül bu pozisyonu üç yönde değerleme çarpanı üreten bir platforma dönüştürür:

1. **Grant / public-good etkisi:** MEV Shield, Liquidity Drain Early Warning ve AI Exploit Simulator; kullanıcı kaybını, rug-pull etkisini ve protokol hack riskini ölçülebilir şekilde azaltır. Bu modüller Solana güvenliği, geliştirici deneyimi ve kullanıcı koruması başlıklarında hibe komitelerine doğrudan raporlanabilir metrik üretir.
2. **B2B SaaS geliri:** Smart Money Oracle, DAO Treasury Guardian ve PoR / Bridge Risk Monitor; fonlar, DAO'lar, market-maker'lar ve borsalar için API, webhook, WebSocket ve white-label dashboard olarak paketlenebilir. Bu yapı aylık abonelik, koltuk bazlı RBAC lisansı ve usage-based API ücretlendirmesi için uygundur.
3. **Derin teknoloji savunulabilirliği:** Jito bundle analizi, Geyser gRPC block parsing, SVM sandbox fuzzing, on-chain clustering ve Merkle Proof of Reserve doğrulama gibi bileşenler; yalnızca UI katmanı olmayan, veri ağı ve risk motoru ağırlıklı bir teknoloji hendekleri seti oluşturur.

### Değerleme etkisi

Bu modüller birlikte Koschei'yi “risk dashboard” kategorisinden “otonom Web3 risk işletim sistemi” kategorisine taşır. Yatırımcı açısından değerleme etkisi aşağıdaki şekilde okunur:

| Değer sürücüsü | Ürün etkisi | Yatırımcı anlatısı | Ölçülebilir KPI |
|---|---|---|---|
| Kullanıcı koruma | MEV ve rug-pull zararını erken önleme | “Koschei kullanıcı varlığını koruyan güvenlik katmanı” | `mev_saved_usd`, `liquidity_loss_prevented_usd`, `alerts_delivered_before_loss` |
| Kurumsal gelir | API, webhook, white-label ve RBAC panel | “Fon/DAO güvenlik ve alpha istihbaratı için yıllık lisans” | `enterprise_mrr_usd`, `api_latency_ms_p95`, `active_institutional_accounts` |
| Ekosistem güvenliği | Protokol fuzzing ve exploit raporları | “Solana geliştiricileri için güvenlik kamu malı” | `critical_findings_pre_launch`, `programs_fuzzed`, `koschei_certified_protocols` |
| Şeffaflık | CEX PoR ve bridge risk skorları | “FTX sonrası proof-of-trust oracle” | `reserve_ratio_delta`, `bridge_risk_score`, `verified_merkle_batches` |

### Ürün ilkeleri

- **No-custody varsayılanı:** Koschei kullanıcıdan private key, seed phrase veya custody yetkisi istemez. Riskten kaçınma API'leri varsayılan olarak alarm üretir; otomatik varlık taşıma yalnızca müşteri tarafında imzalanmış policy ve multisig onayı ile tetiklenir.
- **Türkçe admin zorunluluğu:** Enterprise operasyon paneli, hibe raporları, modül ayarları, müşteri lisansları ve alarm konfigürasyonu Türkçe terimlerle yönetilir.
- **Metrik odaklı mimari:** Her modül, ticari değer ve ekosistem faydasını kanıtlayacak telemetri event'leri üretir.
- **Modüler teslimat:** Her enterprise modül ayrı branch, ayrı mimari dokümanı, unit/integration test ve golden-master finansal simülasyon testleri ile PR'a dönüştürülür.

---

## Sayfa 2 — Faz 1 ve Faz 4: Kullanıcı Koruma Kalkanı

### Faz 1: MEV Shield & Sandwich Attack Predictor

#### Teknik mimari

- **Go MEV worker:** Jito Block Engine'den bundle metadata, tip account hareketleri, slot bazlı bundle yoğunluğu ve MEV pattern'leri toplanır.
- **Swap simülasyon servisi:** Raydium/Jupiter benzeri swap rotaları için input amount, expected output, slippage tolerance, pool depth ve fiyat etkisi simüle edilir.
- **Sandwich risk motoru:** Kullanıcının swap'ı için front-run/back-run kârlılığı, pool likiditesi, slippage açıklığı ve beklenen zarar hesaplanır.
- **Korumalı gönderim önerisi:** Riskli swap için “Jito tip ile korumalı gönder” önerisi üretilir. Koschei imza almaz; kullanıcı kendi cüzdanında onaylar.
- **Admin görünümü:** Türkçe panelde “MEV Önleme”, “Tahmini Kurtarılan Tutar”, “Riskli Swap Oranı” ve “Jito Koruma Kullanımı” kartları yer alır.

#### Veri kaynakları

- Jito bundle stream ve tip account gözlemleri
- Solana RPC / enhanced RPC transaction simulation
- Raydium/Jupiter route ve pool verileri
- Koschei kullanıcı watchlist ve işlem ön-analiz event'leri

#### Hibe ve yatırım KPI'ları

| KPI | Açıklama | Hibe / ticari kullanım |
|---|---|---|
| `mev_saved_usd` | Koruma önerisi kabul edildiğinde tahmini engellenen sandwich zararı | Solana kullanıcı koruma raporu |
| `sandwich_risk_score` | 0-100 arası saldırıya açıklık skoru | Ürün içi premium sinyal |
| `protected_swap_count` | Koruma ile gönderilen swap sayısı | Ekosistem benimseme metriği |
| `avg_detection_latency_ms` | Swap simülasyonu ve risk skoru üretim gecikmesi | Teknik performans kanıtı |

### Faz 4: Real-Time Liquidity Drain Early Warning System

#### Teknik mimari

- **Geyser gRPC consumer:** Validator/Geyser stream üzerinden ham blok, account write ve token program instruction verisi düşük gecikmeyle alınır.
- **Pool state parser:** AMM pool rezervleri, LP mint/burn, remove-liquidity ve büyük tekil transferler normalize edilir.
- **Anomali motoru:** Tek işlemde veya kısa pencere içinde %50+ likidite çekimi, LP token yakımı ve owner authority değişimi tespit edilir.
- **Alarm dağıtımı:** Push notification, Telegram bot ve Twilio SMS kuyruğu asenkron olarak tetiklenir.
- **Admin görünümü:** Türkçe panelde “Kırmızı Alarm”, “Likidite Çekimi”, “Etkilenen Cüzdan Sayısı”, “Kurtarılan Tahmini Portföy” raporları yer alır.

#### Veri kaynakları

- Solana Geyser gRPC / validator stream
- SPL Token ve Token-2022 account değişimleri
- Raydium, Orca ve Meteora pool hesapları
- Koschei watchlist ve portföy tracker varlık eşleşmeleri

#### Hibe ve yatırım KPI'ları

| KPI | Açıklama | Hibe / ticari kullanım |
|---|---|---|
| `liquidity_loss_prevented_usd` | Alarmdan sonra pozisyon azaltan kullanıcılar için tahmini önlenen kayıp | Public-good etki raporu |
| `rug_alert_latency_ms` | Bloktan kullanıcı alarmına uçtan uca gecikme | Teknik üstünlük metriği |
| `wallets_alerted_count` | Riskli token tutan ve uyarılan kullanıcı sayısı | Topluluk koruma metriği |
| `pool_drain_confidence` | Likidite çekimi alarm güven skoru | Premium API sinyali |

---

## Sayfa 3 — Faz 2 ve Faz 5: Kurumsal Gelir Motoru

### Faz 2: Institutional Smart Money & Whale Oracle

#### Teknik mimari

- **On-chain clustering motoru:** Ortak fonlama kaynağı, eş zamanlı işlem, aynı DEX rotası, tekrar eden token sepeti ve CEX hot-wallet etkileşimlerinden wallet cluster çıkarılır.
- **Postgres Recursive CTE / Graph katmanı:** İlk sürümde Postgres CTE ile cluster traversal; ölçek büyüdüğünde graph storage adapter arayüzü.
- **CEX / OTC izleme:** Binance, Coinbase, Wintermute, Jump Trading gibi bilinen etiketli cüzdanlar için inflow/outflow anomalileri takip edilir.
- **Low-latency WebSocket API:** Kurumsal müşteriler için smart-money trade feed, whale alert ve CEX flow event'leri WebSocket ile yayınlanır.
- **Webhook / Telegram alarmı:** Fonlar için policy bazlı alarm kuralları tanımlanır.

#### B2B paketleme

- **API planı:** WebSocket feed + REST historical query + webhook.
- **White-label planı:** Fonun markasıyla cluster dashboard.
- **Enterprise SLA:** p95 latency, uptime raporu ve özel CEX/wallet label listesi.

#### KPI'lar

| KPI | Açıklama | Ticari kullanım |
|---|---|---|
| `smart_money_signal_count` | Yayınlanan kurumsal alpha sinyali sayısı | B2B usage billing |
| `cex_flow_usd_detected` | Borsalara giren/çıkan varlıkların USD hacmi | Fon risk raporu |
| `copy_trade_api_latency_ms_p95` | WebSocket olay dağıtım p95 gecikmesi | SLA kanıtı |
| `institutional_webhook_delivery_rate` | Başarılı webhook teslim oranı | Enterprise güven metriği |

### Faz 5: DAO Treasury & Multisig Guardian

#### Teknik mimari

- **Squads multisig health scanner:** Signer cüzdan yaşı, geçmiş aktivite, ortak cluster bağlantıları ve riskli kontrat etkileşimleri analiz edilir.
- **Insider threat skoru:** Signer yoğunlaşması, dormant signer, yeni eklenen yüksek riskli signer ve CEX/bridge ilişkileri skorlanır.
- **Proposal instruction decompiler:** DAO teklifindeki instruction'lar decode edilir; hazine varlık çıkışı, hedef cüzdan ve beklenen token hareketi simüle edilir.
- **RBAC kurumsal dashboard:** Türkçe arayüzde “Hazine Özeti”, “İmza Sahibi Sağlığı”, “Teklif Risk Analizi”, “Onay Akışı” ve “Denetim Kaydı” ekranları.
- **Audit log:** Her risk skoru, uyarı ve proposal simülasyonu immutable audit event olarak saklanır.

#### B2B paketleme

- **DAO yıllık lisans:** Hazine büyüklüğüne göre fiyatlandırma.
- **Governance risk API:** Proposal öncesi otomatik risk skoru.
- **White-label treasury risk portal:** DAO topluluğuna şeffaflık sayfası.

#### KPI'lar

| KPI | Açıklama | Ticari kullanım |
|---|---|---|
| `treasury_at_risk_usd` | Riskli proposal veya signer nedeniyle tehlikedeki varlık | DAO satış argümanı |
| `proposal_loss_prevented_usd` | Reddedilen/düzeltilen proposal sonrası tahmini önlenen kayıp | Hibe ve case study |
| `signer_risk_score_avg` | Multisig signer risk ortalaması | Yönetim sağlığı raporu |
| `rbac_admin_actions_count` | Kurumsal panelde yapılan yetkili işlem sayısı | Ürün benimseme metriği |

---

## Sayfa 4 — Faz 3 ve Faz 6: Derin Teknoloji ve Şeffaflık Oracle'ı

### Faz 3: AI Exploit Simulator & Smart Contract Fuzzer

#### Teknik mimari

- **SVM sandbox adapter:** Solana BPF bytecode veya program ID üzerinden izole test ortamı hazırlanır.
- **Dinamik fuzzing motoru:** Rastgele ve guided input üretimi; hesap state kombinasyonları, instruction data varyasyonları ve yetki senaryoları çalıştırılır.
- **Vulnerability classifier:** Reentrancy benzeri cross-program invocation riskleri, integer overflow/underflow, authority bypass, unchecked account owner, rent/state corruption ve privilege escalation pattern'leri sınıflandırılır.
- **AI raporlayıcı:** Bulgu, call trace, minimal repro ve önerilen düzeltme Together AI'a özetlenir; geliştiriciye açıklamalı rapor üretilir.
- **Koschei Certified rozeti:** Belirli fuzz coverage ve kritik bulgu eşiğini geçen protokollere doğrulanabilir sertifika metadata'sı verilir.

#### Sigorta entegrasyonu fikri

Koschei, protokol güvenlik skorunu sigorta partnerlerine oracle verisi olarak sağlayabilir. “Koschei Onaylı Güvenli” rozeti kullanan protokoller için prim indirimi veya yönlendirme komisyonu modeli uygulanır. İlk sürümde on-chain komisyon akıllı sözleşmesi yerine imzalı off-chain attestation ve fatura bazlı gelir modeli önerilir; daha sonra programatik settlement eklenir.

#### KPI'lar

| KPI | Açıklama | Hibe / ticari kullanım |
|---|---|---|
| `programs_fuzzed_count` | Test edilen Solana program sayısı | Geliştirici ekosistemi metriği |
| `critical_findings_pre_launch` | Canlıya çıkmadan bulunan kritik açık | Protokol güvenlik kanıtı |
| `fuzz_execs_per_second` | Fuzzing motor performansı | Derin teknoloji metriği |
| `estimated_exploit_loss_prevented_usd` | Bulgu kapatılırsa önlenen tahmini exploit kaybı | Yatırım/hibe raporu |

### Faz 6: Cross-Chain Bridge & CEX Proof of Reserve Monitor

#### Teknik mimari

- **Bridge TVL monitor:** Wormhole, LayerZero ve benzeri köprülerde locked asset değişimi, guardian/relayer davranışı ve contract upgrade event'leri izlenir.
- **Bridge risk skorlayıcı:** Ani TVL düşüşü, upgrade timing, guardian quorum değişimi ve zincirler arası gecikme anomalileri skora dönüştürülür.
- **PoR Merkle doğrulayıcı:** CEX'lerin yayınladığı Proof of Reserves Merkle root ve kullanıcı yükümlülük verileri doğrulanır; asset/liability oranı izlenir.
- **Flight to Safety API:** CEX risk eşiği aşılınca webhook, Telegram veya kurumsal risk motoruna API alarmı gönderilir. Varlık taşıma otomasyonu müşteri tarafında imzalanan policy ile sınırlanır.
- **Admin görünümü:** Türkçe panelde “Borsa Rezerv Sağlığı”, “Köprü Risk Skoru”, “Merkle Doğrulama Durumu” ve “Kurumsal Alarm Politikaları” ekranları.

#### B2B paketleme

- **Exchange risk feed:** Fonlar ve market-maker'lar için CEX risk skor API'si.
- **Bridge exposure dashboard:** Cross-chain protokoller için TVL ve bridge dependency görünümü.
- **Compliance report export:** Denetim ekipleri için PoR doğrulama CSV/JSON/PDF çıktısı.

#### KPI'lar

| KPI | Açıklama | Ticari kullanım |
|---|---|---|
| `verified_merkle_batches_count` | Doğrulanan PoR batch sayısı | Şeffaflık kanıtı |
| `reserve_ratio_delta` | Varlık/borç oranındaki kritik değişim | Risk alarmı |
| `bridge_outflow_anomaly_usd` | Anormal köprü çıkışı USD hacmi | Fon risk sinyali |
| `flight_to_safety_alerts_count` | Kurumsal risk kaçınma alarm sayısı | Enterprise kullanım metriği |

---

## Sayfa 5 — Teslimat Planı, Telemetri Modeli ve PR Stratejisi

### Önceliklendirilmiş teslimat planı

1. **Öncelik 1 — Hibe garantisi / kullanıcı koruma**
   - `feature/module-1-mev-shield`
   - `feature/module-4-liquidity-drain-warning`
   - Çıktılar: Go worker, simülasyon motoru, golden-master testler, Türkçe admin raporları, public impact metrikleri.
2. **Öncelik 2 — Gelir motoru / B2B SaaS**
   - `feature/module-2-smart-money-oracle`
   - `feature/module-5-dao-treasury-guardian`
   - Çıktılar: WebSocket API, webhook alarmı, RBAC dashboard, enterprise audit log, kullanım bazlı metrikler.
3. **Öncelik 3 — Derin teknoloji / güvenlik ve şeffaflık**
   - `feature/module-3-ai-exploit-simulator`
   - `feature/module-6-por-bridge-monitor`
   - Çıktılar: SVM fuzzing adapter, AI raporlayıcı, PoR Merkle doğrulama, bridge risk oracle.

### Ortak telemetri sözleşmesi

Her modül aşağıdaki ortak event alanlarını üretir:

```json
{
  "event_id": "uuid",
  "module": "mev_shield|smart_money|exploit_simulator|liquidity_drain|dao_guardian|por_bridge",
  "tenant_id": "enterprise_or_public_user_scope",
  "chain": "solana|ethereum|bitcoin|cross_chain",
  "risk_score": 0,
  "estimated_value_at_risk_usd": 0,
  "estimated_loss_prevented_usd": 0,
  "latency_ms": 0,
  "confidence": 0,
  "created_at": "RFC3339"
}
```

Bu sözleşme hibe raporlarında toplam etkiyi; enterprise satışta ise müşteri bazlı ROI raporunu hesaplamayı sağlar.

### Test stratejisi

- **Unit test:** Skorlama fonksiyonları, telemetri hesapları, instruction decoder ve Merkle doğrulama gibi deterministik parçalar.
- **Integration test:** RPC/Geyser/Jito/CEX veri kaynağı adapter'ları için mock server ve fixture temelli testler.
- **Golden-master test:** MEV simülasyonu, liquidity drain, proposal asset flow ve reserve-ratio hesapları için sabit fixture ile tekrar üretilebilir finansal doğruluk testi.
- **Load test:** WebSocket copy-trading feed ve alert delivery kuyruğu için p95/p99 latency doğrulaması.
- **Security test:** No-custody sınırı, RBAC yetki izolasyonu, webhook signature validation ve audit log bütünlüğü.

### Türkçe admin panel kapsamı

Admin panelde tüm modüller için Türkçe operasyon ekranları yer alır:

- “Hibe Etki Raporu”
- “Kurumsal Müşteri ve Lisans Yönetimi”
- “MEV Koruma Ayarları”
- “Kırmızı Alarm Kuralları”
- “DAO Hazine Sağlığı”
- “Borsa Rezerv ve Köprü Riski”
- “AI Güvenlik Bulguları”
- “API Anahtarları, Webhook ve SLA İzleme”

### PR stratejisi

Her modül bağımsız PR olacaktır. Her PR açıklamasında Türkçe olarak şu başlıklar bulunur:

1. **Bu modülün projeye katacağı ticari ve ekosistem değeri**
2. Teknik mimari ve veri kaynakları
3. Telemetri ve hibe KPI'ları
4. Test kanıtları ve golden-master fixture'ları
5. Güvenlik / no-custody değerlendirmesi
6. Enterprise paketleme ve fiyatlandırma hipotezi

### Onay bekleyen kararlar

- İlk enterprise hedef segment: DAO hazineleri mi, kripto fonları mı, Solana protokol ekipleri mi?
- İlk gerçek zamanlı veri sağlayıcı tercihi: Jito/Geyser doğrudan entegrasyon mu, enhanced RPC + fixture simülasyonu ile aşamalı başlangıç mı?
- Admin panel kapsamı: Mevcut statik panel genişletmesi mi, ayrı RBAC enterprise console mu?
- Hibe başvurusu önceliği: Solana kullanıcı koruma / MEV mi, geliştirici güvenliği / fuzzing mi?

---

## Yönetici Özeti

Koschei için önerilen altı modül, tek tek özellik değil, birbirini besleyen bir “Web3 Risk Operating System” katmanıdır. MEV Shield ve Liquidity Drain sistemi kullanıcıyı doğrudan korur; Smart Money Oracle ve DAO Guardian ödeme gücü yüksek B2B müşteriler yaratır; AI Exploit Simulator ve PoR / Bridge Monitor ise Koschei'yi derin teknoloji ve şeffaflık oracle'ı olarak konumlandırır.

Bu strateji, grant komitelerine ölçülebilir kamu yararı; VC'lere yüksek marjlı enterprise gelir; kurumsal müşterilere ise para kaybını önleyen, riski azaltan ve karar hızını artıran teknik altyapı sunar.

---

## Sayfa 6 — Faz 1 Teknik Temel Güncellemesi (Owner Merkezi + 6 Enterprise Modül)

### Owner Kontrol Merkezi

Yeni gizli Owner paneli tamamen Türkçe olarak tasarlandı. Panel repoda takip edilmez; `.gitignore` kuralı ile `koschei/api/public/owner.html` gizli kalır. Giriş akışı `OWNER_WALLET` + `OWNER_SECRET` doğrular, backend HTTP-only cookie setler ve frontend tüm `/api/owner/*` çağrılarında `X-Koschei-Secret` header'ını otomatik ekler.

Operasyonel ekranlar:

- Kullanıcı Yönetimi: cüzdan, kayıt tarihi, kredi, aktif/banned/kaldırıldı durumu.
- Shopier Onay Masası: ödeme ID, kullanıcı, paket, tutar, tarih, onay/red.
- AI Komut Terminali: branch oluşturma, kod yazma, test, commit ve PR log sözleşmesi.
- Gerçek Zamanlı Dashboard: aktif kullanıcı, günlük TRY gelir, kurtarılan USD, bekleyen PR.
- Hibe Takipçisi: Solana Foundation, Ethereum Grants ve Phase-2 güvenlik hibeleri.
- DAO Guardian: hazine riski, proposal kaybı ve bekleyen inceleme kartları.

### Enterprise modül değerleme ve gelir modeli

| Modül | İlk teknik temel | Ölçülebilir değer | B2B paket | Aylık hedef fiyat |
|---|---|---:|---|---:|
| MEV Shield & Sandwich Attack Predictor | `/api/v1/mev/analyze`, `mev_protection_events`, TX Decoder uyarı payload'ı | `mev_saved_usd`, `estimated_loss_usd`, `protected_swap_count` | Wallet, DEX ve trading bot API | 2.500-15.000 USD |
| Institutional Smart Money Oracle | `/ws/smart-money`, REST snapshot, `whale_clusters`, `cex_flows` | `net_flow_usd`, `cluster_confidence`, `active_institutional_accounts` | Fon/market-maker WebSocket aboneliği | 5.000-25.000 USD |
| Real-Time Liquidity Drain Radar | `/api/v1/liquidity/analyze`, `liquidity_drain_alerts` | `liquidity_loss_prevented_usd`, `alerts_delivered_before_loss` | Launchpad, DEX ve risk desk webhook'u | 3.000-20.000 USD |
| DAO Treasury Guardian | `/api/v1/dao/proposal-risk`, `dao_treasuries`, `proposal_risks`, owner DAO sekmesi | `treasury_at_risk_usd`, `proposal_loss_prevented_usd` | DAO ve multisig yıllık lisans | 4.000-30.000 USD |
| AI Exploit Simulator | Phase-2 DB şeması: `exploit_simulation_runs`, `exploit_findings` | `critical_findings_pre_launch`, `estimated_exploit_loss_prevented_usd` | Protokol audit ön-tarama | 8.000-40.000 USD |
| Bridge/PoR Monitor | Phase-2 DB şeması: `bridge_risk_events`, `por_monitor_snapshots` | `bridge_outflow_anomaly_usd`, `reserve_ratio_delta` | Fon/CEX risk feed | 6.000-35.000 USD |

### Ekosistem etkisi varsayımları

- MEV Shield ile yüksek slippage ve düşük likidite swap'larında yıllık 500M USD'ye kadar kullanıcı kaybı önlenebilir; Koschei bunu `mev_saved_usd` ve `estimated_loss_usd` üzerinden raporlar.
- Liquidity Drain Radar ilk blokta alarm verirse launchpad/DEX kullanıcıları için rug-pull etkisinin %20-40'ı azaltılabilir; metrik `liquidity_loss_prevented_usd` olarak izlenir.
- DAO Treasury Guardian, proposal simülasyonu ile hazine çıkışlarını oylama öncesi görünür kılar; 10 DAO müşteride 100M+ USD izlenen hazine hedeflenir.
- Smart Money Oracle, kurumsal abonelik tarafında en hızlı nakit akışı üretecek modüldür; WebSocket ve snapshot API aynı veri sözleşmesine bağlanmıştır.
- AI Exploit Simulator ve Bridge/PoR Monitor şimdilik DB temeliyle hazırlanmıştır; Phase-2'de worker, raporlayıcı ve oracle katmanları eklenecektir.

### İlk 12 ay gelir senaryosu

| Senaryo | Kurumsal müşteri | Ortalama MRR | Yıllık gelir |
|---|---:|---:|---:|
| Muhafazakâr | 8 | 4.000 USD | 384.000 USD |
| Baz | 25 | 7.500 USD | 2.250.000 USD |
| Agresif | 60 | 10.000 USD | 7.200.000 USD |

Baz senaryo; 10 fon/market-maker Smart Money aboneliği, 6 DAO Guardian lisansı, 5 MEV Shield entegrasyonu ve 4 Liquidity Radar webhook müşterisi varsayar. Bu dağılım 6M USD enterprise modül hedefi için doğrudan yatırım anlatısı sağlar.
