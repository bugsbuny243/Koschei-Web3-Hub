# KOSCHEI — İŞ 5 (YENİDEN): CLAIM YÜZEYİNİ KEŞFET, VARSAYMA
Tarih: 2026-10-15
Dayanak: zip 63 kaynak kodu okundu. PYUSD taraması (15.10.2026) 12/14 kol üretti; üretmeyen ikisi `walletless_claim_shield` ve `claim_surface_risk`.

## Gerçek durum — tam olarak ne oluyor

İki kol için kodda **iki farklı yol** var ve yanlış olan hangisinin seçildiği.

**Yol A — taslak.** `internal/services/arvis_arms.go:62` ve `:72`:
```go
claimShield  := unavailableArm("Walletless Claim Shield", ModuleWalletlessClaimShield, req, generatedAt,
    "A parsed claim instruction is required; token-holder evidence is not a claim-surface substitute.")
claimSurface := unavailableArm("Claim Surface Risk", ModuleClaimSurfaceRisk, req, generatedAt,
    "URL/domain evidence is required; claim-instruction evidence belongs to Walletless Claim Shield.")
```
Aynısı `arvis_stage_isolated_scan.go:53` ve `:63`'te tekrar ediyor. Bu çağrılar hiçbir sorgu yapmaz; `arm_evidence_available: false` sabitiyle döner.

**Yol B — gerçek kol.** `internal/services/arvis_claim_arms.go`:
- `EnrichArvisBundleWithClaimSurface` (satır 32) — `security_radar_evidence_gate.go:35`'te çağrılıyor
- `parseArvisClaimSurface(raw string)` (satır 65)
- `buildWalletlessClaimArm` (satır 119) ve `buildClaimSurfaceArm` (satır 139)
- `matchedClaimTerms` (satır 191) — `claim`, `airdrop`, `reward`, `bonus`, `mint`, `whitelist`, `presale`, `verify` terimlerini eşleştiriyor

**Sorun:** Yol B yalnızca **çağıran taraf bir claim yüzeyi dizesi (URL) verdiğinde** devreye giriyor. Düz bir mint taramasında — PYUSD'de olduğu gibi — kimse URL vermiyor, taslaklar yerinde kalıyor ve rapor "uygulanamaz" yazıyor.

Yani eksik olan analiz değil, **keşif**. Motor "bu tokende claim yüzeyi var mı" diye hiç sormuyor; birinin gelip söylemesini bekliyor.

## Taranabilir mi — evet, malzeme de kısmen hazır

**Zincir üstü:** bir claim/airdrop dağıtımı varsa token, bir dağıtıcı ya da vesting programının sahip olduğu hesapta durur. Streamflow programının kimliği kodda **zaten tanımlı**: `internal/handlers/lp_market_context.go:25` → `strmRqUCoQUgGUan5YhzUZa6KqdzwX5L6FpUxfmKg5m`.

**Zincir dışı:** `internal/radaradapter/frontend` paketi hazır — TLS sertifika parmak izi, HTML/JS SHA-256, DNS ve CT gözlemi, yakalanmış cüzdan isteği. Mint taramasına bağlı değil.

**Terim analizi:** `matchedClaimTerms` yazılmış, yalnızca URL verildiğinde çalışıyor.

## Yapılacak

### 5.1 Zincir üstü claim keşfi — yeni toplayıcı

`internal/services/claim_surface_discovery.go` yazılacak. Mint için şunları arar:

1. **Dağıtıcı/vesting programlarının elindeki token hesapları.** `getTokenLargestAccounts` ve holder kümesindeki her hesabın `owner` programına bakılır; sahibi bilinen bir dağıtıcı programsa kanıt satırı üretilir.
2. **Creator/mint authority'nin program PDA'larına çıkışları.** Mevcut `actor_initial_recipients` ve `actor_funding_origin` yolları zaten imza ayrıştırıyor; hedefi program sahipli bir hesapsa işaretlenir.
3. **Mint ile aynı işlemde görünen program kimlikleri.** `program_relation_scan` zaten bu veriyi topluyor; dağıtıcı listesiyle kesiştirilir.

**Bilinen dağıtıcı program listesi** tek bir yerde, adıyla ve kaynağıyla tanımlanacak:
```go
// Her kimlik, zincirden okunmuş bir hesabın owner alanıyla doğrulanacak.
// Ezbere program kimliği yazılmayacak; her biri bir fixture ile kanıtlanacak.
var knownClaimDistributorPrograms = map[string]string{
    "strmRqUCoQUgGUan5YhzUZa6KqdzwX5L6FpUxfmKg5m": "streamflow",
    // merkle distributor, jupiter distributor, vb. — her biri fixture ile
}
```
Listeye eklenen her program için gerçek bir hesap yanıtından üretilmiş fixture zorunlu.

### 5.2 Kolları keşfe bağla

`arvis_arms.go:62` ve `:72` ile `arvis_stage_isolated_scan.go:53` ve `:63`'teki `unavailableArm` çağrıları kaldırılacak. Yerine 5.1'in çıktısı geçecek.

`buildWalletlessClaimArm` ve `buildClaimSurfaceArm` iki girdi kabul edecek: zincir üstü keşif kanıtı ve (varsa) çağıranın verdiği URL. Biri yoksa diğeriyle çalışacak.

`EnrichArvisBundleWithClaimSurface` korunacak — URL geldiğinde zincir üstü kanıtın **üstüne** eklenecek, yerine geçmeyecek.

### 5.3 Zincir dışı yüzey, kaynağı varsa

Token metadata'sının `uri` alanı ya da proje sitesi bir origin veriyorsa, `internal/radaradapter/frontend` toplayıcısı o origin için çalıştırılacak ve `matchedClaimTerms` sayfa metnine uygulanacak.

Origin yoksa bu **uydurulmayacak**: `no_declared_origin` gerekçesiyle ayrı bir sınır satırı yazılacak.

### 5.4 Bulamadığında ne diyecek — kanıtlı olumsuz

Hiçbir dağıtıcı bulunamazsa kol şunu üretecek:

```
reason_code:   no_known_distributor_program_found
searched:      <taranan program listesinin adları ve sayısı>
scope:         token_largest_accounts + creator_outbound_transfers + program_relations
observed_slot: <slot>
limitation:    UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT
```

Metin açıkça şunu söyleyecek: *"Bilinen N dağıtıcı program arasında claim yüzeyi bulunmadı, slot X itibarıyla. Tanınmayan özel bir dağıtıcı program bu gözlemle dışlanamaz."*

Evrensel olumsuz iddia edilmeyecek. Sınır yazılacak.

### 5.5 İki kol aynı şeyi söyleyecek

`walletless_claim_shield` ve `claim_surface_risk` aynı keşif çıktısını kullanacak. Biri "uygulanamaz" diğeri "kanıt eksik" diyemeyecek. İkisi de ya kanıt üretecek ya da aynı `reason_code` ile aynı kanıtlı olumsuzu verecek.

### 5.6 Hiçbir skor üretilmeyecek

Claim yüzeyi bulunması tek başına risk değildir — meşru airdrop'lar da dağıtıcı program kullanır. Kol **gözlem** üretir, not vermez: hangi program, hangi hesap, ne kadar, hangi slot. Yorum kural motoruna aittir.

## CI korumaları

1. **`TestClaimArmsAreNotHardcodedUnavailable`** — `arvis_arms.go` ve `arvis_stage_isolated_scan.go` içinde `ModuleWalletlessClaimShield` veya `ModuleClaimSurfaceRisk` için `unavailableArm` çağrısı bulunursa build kırılır.
2. **`TestClaimDiscoveryProducesEvidenceOrBoundedNegative`** — fixture ile: dağıtıcı bulunan hedefte kanıt satırı üretilir; bulunmayan hedefte `reason_code`, `searched`, `observed_slot` ve `UNKNOWN_CUSTOM_DISTRIBUTOR_NOT_RULED_OUT` sınırı dolu döner.
3. **`TestClaimArmsAgree`** — iki kol aynı hedefte aynı durumu ve aynı `reason_code`'u verir.
4. **`TestDistributorProgramsHaveFixtures`** — `knownClaimDistributorPrograms` içindeki her kimlik için fixture dosyası bulunmalı.

## Kabul kriteri

PYUSD yeniden tarandığında:
- İki kol da "uygulanamaz" demez.
- Dağıtıcı bulunmadıysa kaç program arandığını, hangi kapsamda ve hangi slot'ta arandığını yazar.
- "Tanınmayan özel dağıtıcı dışlanamaz" sınırı raporda görünür.
- Kol sayısı 12/14'ten 14/14'e çıkar ya da çıkmayan kolun gerekçesi adıyla yazılır.

## Değişmez kurallar

Dokunulmayacak dosyalar listesi geçerlidir. Hiçbir dosya ve hiçbir veritabanı satırı silinmeyecek. `SolanaMaxSupportedTransactionVersion` 0 kalacak. Her adımdan sonra `gofmt -l . && go vet ./... && go test ./...`; rota değişirse `go run ./cmd/openapi-gen`.