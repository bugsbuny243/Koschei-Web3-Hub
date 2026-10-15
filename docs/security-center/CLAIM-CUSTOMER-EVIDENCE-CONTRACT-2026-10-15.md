# KOSCHEI — İŞ 6: MÜŞTERİ KANIT YÜZEYİ
Tarih: 2026-10-15
Dayanak: zip 63 kaynağı okundu. Rota ve payload yapıları aşağıda satır numarasıyla.

## Ölçüm — müşteri bugün ne görüyor

Dört ayrı müşteri yüzeyi var ve her biri farklı şey anlatıyor.

| Rota | Kapı | `arms` | Kol kapsamı | Kullanan |
|---|---|---|---|---|
| `/api/owner/arvis/scan`, `/api/owner/radar/unified` | `ownerOnly` (`server.go:166-167`) | tam 14 kol | var | yalnız owner |
| `/api/v1/radar/check` | `planTier("professional")` (`server.go:219`) | **var** | `investigation_report.investigation_coverage` | professional müşteri |
| `/api/token/scan` | `risk` (`server.go:211`) | **yok** | var ama `investigation_report` içine gömülü | en çok referanslanan rota (7 yer) |
| `/api/scan` | kapı yok (`customer_scan_routes.go:47`) | **yok** | **yok** | **ana müşteri UI'ı** (`customer-universal-address-scan-v1.js`) |
| `/api/v1/risk/badge` | `badge` (`server.go:216`) | yok | yok, yalnız `verified_arm_count` | badge gömenler |

`/api/scan`'in döndürdüğü yapının tamamı — `internal/services/customer_scan_result.go`:
```go
type CustomerScanResult struct {
	Target, Status, Verdict, EvidenceStatus
	Trust, Reasons, EvidenceRefs
	EVMAuthority, InvestigationPlan, TransactionEvidence, AssetEvidence
}
```
Onbir alan. Kol yok, kapsam yok, hangi kolun neden çalışmadığı yok.

**Sonuç:** İş 5'te claim kollarını kusursuz yazsak bile, ana müşteri UI'ında **hiçbir şey değişmez**. Çıktı en iyi hâlde `Reasons` içinde bir dize olur. Kanıtlı olumsuz — "bilinen N dağıtıcı program arasında bulunmadı, slot X itibarıyla" — müşteriye hiç ulaşmaz.

## İkinci bulgu — aynı ürün iki farklı şey söylüyor

`/api/token/scan` müşteriye canlı bir **sayısal skor** döndürüyor (`token_scanner.go:204` → `Score: score`).

Aynı anda ARVIS katmanı her kola `numeric_score_disabled: true` damgası basıyor (`arvis_arms.go:441`, `:461`, `:484`; `arvis_claim_arms.go:177`), ve müşteri zarfı `numeric_final_score_disabled: true` ilan ediyor (`customer_investigation_response.go`).

Yani politika "sayısal skor kapalı" diyor, en çok kullanılan müşteri rotası sayı döndürüyor. Bir denetçi bunu on dakikada bulur.

## Yapılacak

### 6.1 Tek kapsam sözleşmesi — her müşteri yüzeyinde, üst seviyede

`ArvisArmCoverage` zaten var (`arvis_investigation_coverage.go`) ve `BuildArvisInvestigationCoverage` çağrılıyor (`unified_investigation_report.go:409`). Eksik olan onu müşteriye taşımak.

Her müşteri yanıtına **üst seviyede** şu blok eklenecek — `investigation_report` içine gömülmeyecek:

```json
"coverage": {
  "arms_total": 14,
  "arms_completed": 12,
  "arms_withheld": 2,
  "withheld": [
    {
      "module_id": "walletless_claim_shield",
      "module": "Walletless Claim Shield",
      "execution_status": "not_present",
      "reason_code": "no_known_distributor_program_found",
      "searched": "11 pinned distributor/vesting programs",
      "scope": "token_largest_accounts + creator_outbound_transfers + program_relations",
      "observed_slot": 457310747,
      "limitation": "UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT"
    }
  ]
}
```

Kural: `arms_total` her zaman yazılacak. Çıplak bir `verified_arm_count` — paydası olmayan bir sayı — hiçbir yüzeyde tek başına durmayacak.

### 6.2 `/api/scan` kapsamı taşıyacak

`CustomerScanResult`'a iki alan eklenecek:
```go
Coverage *ArvisCoverageSummary `json:"coverage,omitempty"`
ArmCount *ArmCountSummary      `json:"arm_count,omitempty"`
```
Mevcut onbir alan **korunacak**, hiçbiri kaldırılmayacak — `customer_owner_surface_v3_test.go` ve `customer_scan_routes_test.go` geçmeye devam edecek.

Solana token hedefi için `/api/scan` hâlihazırda ARVIS'i çağırmıyorsa, kapsam `null` kalacak ve `status` alanı bunu açıkça söyleyecek: `"coverage_not_requested_on_this_route"`. Sessiz boşluk bırakılmayacak.

### 6.3 `/api/token/scan`'de `arms` ve kapsam yüzeye çıkacak

`tokenScanResponse`'a `Arms` ve `Coverage` alanları eklenecek. `assembly.Report` zaten elde (`token_scanner.go:234`); kapsam oradan çıkarılıp üst seviyeye kopyalanacak.

### 6.4 Skor çelişkisi kapatılacak

İki seçenekten biri, ikisinin arası yok:

**(a)** `tokenScanResponse.Score` kaldırılacak ve yerine `risk_level` + kanıt kalacak. Bu, politikaya uyan yol.
**(b)** `Score` kalacaksa, ne olduğu alan adında yazılacak — `token_extension_penalty_score` gibi — ve yanına `"not_a_final_risk_score": true` ile `"final_score_disabled_by_policy": true` konulacak.

**Karar senin.** Kaldırmak API'yi kıracaksa (b) daha az riskli; ama o zaman pazarlama metninde "skor vermiyoruz" cümlesi de düzeltilecek. İkisinden biri seçilmeden bu iş kapanmış sayılmayacak.

### 6.5 Müşteri kararı değişmeyecek

Kapsam bloğu **bilgi**dir, not değildir. `final_verdict.recommendation` bu bloktan etkilenmeyecek. İki kol kanıt üretmediğinde karar yine `WITHHOLD` tarafında kalacak; fark, müşterinin artık **neyin aranıp bulunmadığını** görmesi.

### 6.6 SDK ve şema

`sdk/typescript` ve `oss/schemas/signed-verdict.schema.json` yeni `coverage` bloğunu tanıyacak. Şema değişirse `go run ./cmd/openapi-gen` çalıştırılacak.

## CI korumaları

1. **`TestEveryCustomerSurfaceCarriesArmCount`** — `/api/scan`, `/api/token/scan`, `/api/v1/radar/check` yanıtlarında `arms_total` bulunmalı; yoksa build kırılır.
2. **`TestWithheldArmCarriesReasonAndScope`** — `withheld` listesindeki her girdi `reason_code`, `searched`, `observed_slot` ve `limitation` alanlarını dolu taşımalı. Boş dize kabul edilmez.
3. **`TestNoBareVerifiedArmCount`** — yanıtta `verified_arm_count` varsa yanında `arms_total` da olmalı.
4. **`TestNumericScorePolicyIsConsistent`** — `numeric_final_score_disabled: true` döndüren bir yanıt, aynı gövdede adı `score` olan ve sıfırdan farklı bir sayı taşıyamaz. 6.4(b) seçilirse alan adı `*_penalty_score` kalıbında olmalı.
5. **`TestCustomerSurfaceFieldsPreserved`** — `CustomerScanResult`'ın mevcut onbir alanı silinmemiş olmalı.

## Kabul kriteri

PYUSD'yi müşteri rotasından tarattığında:
- Kaç kolun çalıştığını ve kaçının kanıt üretmediğini **payda ile** görüyor.
- Üretmeyen kol için ne arandığını, hangi kapsamda ve hangi slot'ta arandığını görüyor.
- "Tanınmayan özel dağıtıcı dışlanamaz" sınırı müşteri yanıtında var.
- Hiçbir yüzeyde politikayla çelişen bir sayı yok.

## Sıralama

İş 5 ile İş 6 birlikte anlam taşıyor. İş 5 tek başına girerse owner radarı düzelir, müşteri hiçbir şey görmez. İş 6 tek başına girerse boş bir kapsam bloğu yayınlanır. **İkisi aynı PR'da gitmeli** ya da İş 6 hemen arkasından.

## Değişmez kurallar

Dokunulmayacak dosyalar listesi geçerlidir — `public/js/koschei-auth.js`, `public/index.html`, `public/js/cipher.js`, `public/js/unified-scan-navigation.js`, `public/js/owner-command-center-v2.js`, `public/js/owner-unified-radar.js`, `public/js/arvis-canonical-projection-v1.js`, `internal/handlers/actor_defense_*.go`, `internal/http/defense_routes.go`. Hiçbir dosya ve hiçbir veritabanı satırı silinmeyecek. Mevcut yanıt alanları kaldırılmayacak, yalnızca eklenecek (6.4 hariç — o karar senin). `SolanaMaxSupportedTransactionVersion` 0 kalacak. Her adımdan sonra `gofmt -l . && go vet ./... && go test ./...`; rota veya şema değişirse `go run ./cmd/openapi-gen`.