# Koschei Web3 Güvenlik Merkezi / ARVIS — teslim programı

**Ürün sınırı (güncel kullanıcı talimatı):** Güvenlik ürünü Koschei Web3 Güvenlik Merkezi / ARVIS'tir. Ayrı bir dil, ajan markası veya bağımsız güvenlik ürünü varsayılmaz. Aşağıdaki kontrol başlıkları yetenek alanlarıdır; yeni ürün anlamına gelmez.

**Kaynak sözleşme:** [34 alan / 170 kontrol / 8 ortak kabul](WSC-34-AREAS-170-CONTROLS-2026-10-15.md). Bu, tamamlanan modüllerin listesi **değildir**. **Hiçbir kontrol yalnızca kodun varlığına, yeşil CI'ya veya bir modül adına dayanılarak VERİFİED ilan edilmez.** Her alanın kanıtı ayrı canlı rota + fixture + gözlem kaynağı + sürüm + inceleme kaydı gerektirir.

## Kaybın görüldüğü saldırı aileleri → koruma iş akışı

| Saldırı ailesi | Başlangıç kontrolleri | Üretim kabul örneği |
| --- | --- | --- |
| Seed/anahtar/ekip hesabı ele geçirme | WSC-15, WSC-23, WSC-25, WSC-27, WSC-31 | Müşteri seed paylaşmadan, yetki envanteri ve olay müdahale koşulları gösterilir; anahtar ihlali zincirden kesin çıkarsanmaz |
| Drainer, sahte claim, izin tuzağı | WSC-15, WSC-16, WSC-17, WSC-24 | Gerçek imzalanacak baytlar çözümlenir; izin farkı ve gerektiğinde **entegrasyon yolunda** BLOCK makbuzu görünür |
| Rug pull, honeypot, LP manipülasyonu | WSC-04–13 | Owner, LP, kilit, satılabilirlik simülasyonu ve gerekçeli WITHHOLD raporu |
| Kontrat ve Solana program hatası | WSC-18–20 | Yetkili izole statik/dinamik analiz, bulgu-fixture ve düzeltme regresyonu |
| Köprü, oracle, flash loan / ekonomik güvenlik | WSC-21–22 | İki zincir kanıtı, gözlem penceresi, fiyat/oracle sağlamlığı; varsayım açık |
| Adres zehirleme / taklit token | WSC-04, WSC-15, WSC-24 | Kaynak ve adres benzerliği, sıfır değerin belirsiz olmaması, resmî mint kanıtı |
| DAO, multisig, insider, proxy yetkisi | WSC-05, WSC-23 | Eşik/rol, upgrade path, timelock, büyük transfer ve olay günlüğü |
| MEV ve piyasa davranışı | WSC-13–14 | İşlem sırası ve ekonomik etkiyle kanıt; verisiz 'MEV engellendi' sözü yok |
| Token-2022 özel kontroller | WSC-06, WSC-19 | Etkin uzantı, delegate ve transfer-hook simülasyonu; unresolved → WITHHOLD |
| Sürekli SOC, tehdit analizi ve uyarı | WSC-26–28 | Kayıpsız izleme, teslim makbuzu, worker heartbeat, vaka kapanışı |
| ARVIS ajan ve çoklu ağ güvenliği | WSC-32, WSC-34 | Yalıtılmış ajan, prompt-injection reddi, ağ bazlı destek sınırları |

## Beş ayrı yürütme aşaması (170 kontrolün tümü tek kez sahiplenilir)

1. **Çekirdek doğruluk ve kanıt (35 kontrol):** WSC-01–03, 29–31, 33. Kanonik delil ve müşteri/API görünümü.
2. **Cüzdan/drainer/önleme/SOC (35 kontrol):** WSC-15–17, 24–27. İmza öncesi analiz, zararlı izin, claim, alarm, olay müdahalesi.
3. **Token/Solana/rug/MEV (55 kontrol):** WSC-04–14. Yetki, holder, lansman, LP, exit simulation, piyasa ve MEV.
4. **Kod/DeFi/köprü/DAO (30 kontrol):** WSC-18–23. Statik+dynamik test, oracle ve köprü güvenlik denetimi.
5. **ARVIS istihbarat/otomasyon/ek zincir (15 kontrol):** WSC-28, 32, 34. Vaka kanıtı, ARVIS araç sınırları ve ağ uzmanlıkları.

Her aşama **G01–G08** kriterlerine uyar. İlgili zincir adaptörü yoksa `unsupported`, izin yoksa `not_run`, eksik veri varsa `partial/unavailable`; hiçbir durum `none_observed` veya `allow` ile karıştırılmaz.

## Kanıt borcu: önceki İş 5 ve İş 6

Önceki #1379 PR'ı canlı çalışan 170 kontrol demek değildir. Claim keşfi en büyük holder'ların pinned program kimliğiyle sınırlıdır; creator outbound, tam program ilişkileri, slot'a bağlı negatif kanıt, metadata URL, gerçek dağıtıcı fixture ve canlı PYUSD kabulü eklenmelidir. Müşteri kapsamı tüm rotalarda ve SDK'da tekrarlanabilir biçimde eşitlenmelidir. Varlığı doğrulanmayan claim yüzeyi 'safe' değildir.

## Değişmez mühendislik kuralları

- Seed veya private key toplanmaz; yazılım kontrolündeki **imza/yayın** işlemleri ayrıca insan onayı ister.
- Gerçek kullanıcı fonlarıyla riskli test yapılmaz; statik, local fork veya simülasyon tercih edilir.
- Korunan public JS, auth ve actor-defense dosyaları, DB satırları, token state ve canlı cüzdanlar dokunulmaz.
- `SolanaMaxSupportedTransactionVersion=0` korunur.
- Kod iddiaları canlı endpoint, kaynağa bağlı delil, CI ve negatif/normal fixture ile kanıtlanır.
- Her PR sonunda `gofmt -l .`, `go vet ./...`, `go test ./...`; şema/rota değiştiğinde OpenAPI yeniden üretilir ve müşteri uyumluluğu test edilir.
