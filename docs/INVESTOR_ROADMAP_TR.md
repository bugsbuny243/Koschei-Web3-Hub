# Koschei Web3 Hub — $1M Yatırım Hazırlık Yol Haritası

Bu doküman Koschei Web3 Hub'ın mevcut MVP durumunu yatırım yapılabilir bir Web3 intelligence şirketine dönüştürmek için 12 aylık uygulanabilir planı listeler. Hedef; **no-custody, read-only, güvenlik odaklı Solana-first intelligence platformu** konumlandırmasını netleştirip ölçülebilir traction, gelir, güven ve teknik derinlik üretmektir.

> Not: Bu bir yatırım tavsiyesi değildir; ürün, büyüme ve fon toplama hazırlık planıdır.

## 1. Mevcut Durum Analizi

### Güçlü Yanlar

- **Net güvenlik pozisyonu:** Ürün özel anahtar, seed phrase veya fon saklama istemeyen no-custody bir yaklaşım üzerine kurulu.
- **Çalışan MVP:** Backend Go, statik frontend, auth, admin analytics, kredi sistemi ve çok sayıda intelligence modülüyle canlıya çıkmaya uygun bir ürün temeli var.
- **Geniş ürün yüzeyi:** Wallet Score, Token Scanner, Risk Scanner, TX Decoder, Metadata Studio, Watchlist, Chain Health, Funding Assistant, Agent API ve pay-per-tool gibi farklı kullanıcı/persona giriş noktaları mevcut.
- **Public-good hikayesi:** Kullanıcıların token, wallet, transaction ve proje risklerini anlamasına yardım eden eğitim/güvenlik odağı yatırımcı ve grant anlatısına uygun.
- **B2B/API potansiyeli:** Agent API, tool usage logs, pricing ve credits yapıları geliştirici/API gelir modeline genişleyebilir.

### Zayıf Yanlar / Yatırım Öncesi Riskler

- **Odak dağınıklığı:** Çok sayıda modül var; yatırımcı için tek cümlelik ana wedge daha net olmalı.
- **Traction kanıtı eksikliği:** Aktif kullanıcı, retention, conversion, API usage ve revenue metrikleri düzenli raporlanmalı.
- **Veri derinliği farkı:** Risk skorlarının kanıt, kaynak, confidence ve benchmark setleri daha görünür olmalı.
- **Marka/domain uyumu:** Proje adı Koschei iken canlı demo domain'i farklı görünüyor; güven için domain/marka bütünlüğü güçlendirilmeli.
- **Kurumsal güvenlik paketi:** Security policy var ancak audit, abuse prevention, rate limiting ve data handling anlatısı yatırım materyaline taşınmalı.

## 2. Milyon Dolarlık Yatırım İçin Ana Tez

Koschei'nin yatırım tezi şu şekilde sıkıştırılmalı:

**"Koschei, Solana kullanıcıları ve builder'ları için no-custody Web3 risk intelligence katmanıdır; public on-chain veriyi okunabilir risk skorlarına, transaction açıklamalarına ve geliştirici API'lerine dönüştürür."**

Yatırımcının görmek isteyeceği 5 kanıt:

1. **Keskin problem:** Web3 kullanıcıları sahte token, rug, sybil, riskli wallet ve anlaşılmayan transaction sorunları yaşıyor.
2. **Canlı çözüm:** En az 3 temel araç hızlı, anlaşılır ve demo edilebilir olmalı: Token Scanner, Wallet Score, TX Decoder.
3. **Traction:** Haftalık aktif kullanıcı, scan sayısı, tekrar kullanım, API key kullanımı ve ödeme isteği artmalı.
4. **Gelir yolu:** Freemium + pay-per-tool + developer API + ekip planı net fiyatlanmalı.
5. **Defensibility:** Risk graph, labeled wallet/event dataset, scoring methodology ve ecosystem integrations zamanla veri hendekleri oluşturmalı.

## 3. 12 Aylık Yol Haritası

### Faz 0 — İlk 2 Hafta: Yatırımcıya Hazır Netlik

- [ ] Ana positioning'i tüm sayfalarda tekleştir: "No-custody Web3 Risk Intelligence for Solana".
- [ ] Landing page'de 3 ana demo CTA belirle: Token Scanner, Wallet Score, TX Decoder.
- [ ] Public Impact sayfasını yatırımcı metrik paneline dönüştür: outputs, scans, active users, API calls, conversion.
- [ ] README, docs ve pitch metninde domain/marka tutarlılığı sağla.
- [ ] Yatırım veri odası klasörü oluştur: pitch deck taslağı, metrics snapshot, product screenshots, API docs, security notes.

### Faz 1 — 0-30 Gün: MVP'yi Yatırım Demo Ürününe Çevir

- [ ] Token Scanner sonuçlarına `risk_factors`, `evidence`, `confidence`, `next_steps` alanları ekle.
- [ ] Wallet Score için davranış kategorileri ekle: new wallet, high-risk interactions, concentrated activity, bot/sybil patterns.
- [ ] TX Decoder için basit dilde "what happened / risk / recommendation" çıktısı üret.
- [ ] Her araçta örnek veriyle tek tık demo ekle.
- [ ] Kullanıcı event tracking'i funnel bazlı standardize et: visit → scan_start → scan_success → signup → payment_request.
- [ ] Admin analytics'te haftalık metrik snapshot endpoint'i oluştur.
- [ ] Minimum rate limiting, abuse guard ve API key quota sistemi netleştir.

**Başarı kriteri:** Haftalık 100+ tamamlanmış scan, 20+ kayıtlı kullanıcı, 5+ tekrar kullanıcı, 3+ pilot görüşmesi.

### Faz 2 — 31-90 Gün: Traction ve Gelir Kanıtı

- [ ] Freemium limitlerini yayınla: ücretsiz günlük scan limiti, Pro credits, API credits.
- [ ] Pay-per-tool akışını production-ready yap: receipt, entitlement, credit event, failed payment handling.
- [ ] Developer API için hızlı başlangıç rehberi ve örnek SDK snippet'leri yayınla.
- [ ] 10 Solana founder/builder ile pilot yap; feedback'i issue/roadmap'e bağla.
- [ ] Weekly public impact raporu yayınla.
- [ ] Grant fırsatları için Funding Assistant çıktısını gerçek başvuru paketine dönüştür.
- [ ] Product analytics ile cohort retention ölç: D1/D7 tekrar kullanım.

**Başarı kriteri:** 500+ weekly scans, 100+ kayıtlı kullanıcı, 10+ pilot kullanıcı, ilk ödeme veya LOI, 2-3 grant başvurusu.

### Faz 3 — 3-6 Ay: Yatırım Turu Hazırlığı

- [ ] Scoring methodology belgesi yayınla: sinyaller, ağırlıklar, confidence, false positive yönetimi.
- [ ] Labeled risk dataset başlat: known scam/rug tokens, trusted projects, high-risk wallets.
- [ ] Intelligence Graph'i ürünün ana farkı yap: wallet-token-project ilişkileri.
- [ ] Team/Pro planlarını netleştir: seat, API quota, watchlist alerts, export.
- [ ] CRM pipeline kur: 100 investor, 50 ecosystem partner, 30 pilot lead.
- [ ] Pitch deck tamamla: problem, solution, market, traction, product, business model, moat, ask, use of funds.
- [ ] Güvenlik ve legal hazırlık: Terms, Privacy, disclaimer, data processing, incident response.

**Başarı kriteri:** $1K-$5K MRR veya eşdeğer LOI, 2K+ weekly scans, 30%+ weekly returning usage, 5+ API/pilot müşteri.

### Faz 4 — 6-12 Ay: Ölçek ve Fon Toplama

- [ ] Solana-first derinliği koruyup EVM/Base/Ethereum için cross-chain beta aç.
- [ ] Watchlist alert sistemi ekle: email/webhook/agent notification.
- [ ] API marketplace ve agent integrations başlat.
- [ ] Ecosystem partnership: wallet, launchpad, analytics, security community.
- [ ] Seed/pre-seed yatırım turu aç: $1M hedef, 18 aylık runway, ürün+data+growth kullanımı.
- [ ] Aylık investor update yayınla: metrics, product shipped, revenue, pipeline, asks.

**Başarı kriteri:** $10K+ MRR veya güçlü enterprise/API LOI, 10K+ weekly scans, ölçülebilir retention, partner integration, yatırım görüşmelerinde lead investor.

## 4. Önceliklendirilmiş İş Listesi

### P0 — Hemen Başlanacaklar

1. **Investor readiness dokümanı ve README bağlantısı** — mevcut çalışma bu maddeyi başlatır.
2. **Metrics snapshot** — admin/public impact verilerini haftalık yatırımcı raporuna uygun hale getir.
3. **3 demo flow** — Token Scanner, Wallet Score, TX Decoder için örnek veri ve net çıktı formatı.
4. **Positioning update** — landing, docs, pricing ve impact sayfalarında tek mesaj.
5. **Pricing cleanup** — ücretsiz/pro/API paketlerinin limitleri ve değer önerisi.

### P1 — Ürün Derinliği

- Evidence-backed risk outputs.
- Confidence score ve explanation standardı.
- Watchlist alerting.
- API key quota ve usage dashboard.
- Exportable reports for teams/builders.

### P2 — Büyüme ve Fonlama

- Grant pipeline ve başvuru takvimi.
- Pilot kullanıcı listesi ve feedback formu.
- Public changelog + weekly build updates.
- Investor CRM ve data room.
- Partner outreach kit.

## 5. Yatırım Kullanım Planı ($1M Örnek)

| Alan | Yaklaşık Pay | Amaç |
| --- | ---: | --- |
| Engineering | 40% | Risk engine, graph, API, reliability, integrations |
| Data & Infrastructure | 20% | RPC/API maliyetleri, labeled datasets, monitoring |
| Growth & Partnerships | 15% | Solana builder outreach, pilots, content, ecosystem relations |
| Security & Compliance | 10% | Audit, policies, abuse prevention, legal docs |
| Design & Product | 10% | UX, reporting, onboarding, demo flows |
| Buffer | 5% | Operasyonel esneklik |

## 6. Haftalık Çalışma Ritmi

- **Pazartesi:** Metrics review + bu haftanın tek ana hedefi.
- **Salı-Çarşamba:** Ürün geliştirme ve test.
- **Perşembe:** Kullanıcı/pilot görüşmeleri, feedback triage.
- **Cuma:** Public changelog, investor update draft, demo kaydı.
- **Hafta sonu:** Grant/pitch materyali ve içerik.

## 7. İlk Çalışma Adımı

Bu PR ile ilk adım olarak yatırım yol haritası dokümante edildi ve README'den erişilebilir hale getirildi. Sıradaki önerilen kod işi: Public Impact/Admin Analytics tarafına haftalık yatırımcı metrik snapshot'ı eklemek ve landing page'de üç ana demo akışını öne çıkarmak.
