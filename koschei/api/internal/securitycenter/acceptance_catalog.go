// Package securitycenter maintains a read-only, evidence-aware catalog of security capabilities.
package securitycenter

import (
	"fmt"
	"strconv"
	"strings"
)

// AcceptanceCatalogSchemaVersion describes requirements, not implementation
// coverage. A listed check is NOT a claim that a live collector exists.
const AcceptanceCatalogSchemaVersion = "koschei.security-center-acceptance.v1"

const (
	AcceptanceUnverified  = "evidence_not_verified"
	AcceptanceNotAssessed = "not_assessed"
)

type AcceptanceArea struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Lead       string `json:"lead"`
	Phase      int    `json:"phase"`
	CheckCount int    `json:"check_count"`
}

type AcceptanceCheck struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	AreaID        string   `json:"area_id,omitempty"`
	Lead          string   `json:"lead"`
	Phase         int      `json:"phase"`
	Applicability string   `json:"applicability"`
	Verification  string   `json:"verification"`
	EvidenceRefs  []string `json:"evidence_refs"`
}

type AcceptanceCatalog struct {
	SchemaVersion   string            `json:"schema_version"`
	SourceDocument  string            `json:"source_document"`
	ScopeNote       string            `json:"scope_note"`
	AreaCount       int               `json:"area_count"`
	ControlCount    int               `json:"control_count"`
	CommonGateCount int               `json:"common_gate_count"`
	VerifiedCount   int               `json:"verified_count"`
	Areas           []AcceptanceArea  `json:"areas"`
	Checks          []AcceptanceCheck `json:"checks"`
}

type requirementDef struct {
	id          string
	description string
}
func checkDef(id, description string) requirementDef {
	return requirementDef{id: id, description: description}
}

var acceptanceAreas = []AcceptanceArea{
	{ID: "01", Name: "Hedef envanteri ve tehdit modeli"},
	{ID: "02", Name: "Zincir verisi, RPC ve indeksleme"},
	{ID: "03", Name: "Kanıt bütünlüğü ve izlenebilirlik"},
	{ID: "04", Name: "Token kimliği ve ekonomik bağlam"},
	{ID: "05", Name: "Yönetim yetkileri ve kontrol haritası"},
	{ID: "06", Name: "Solana Token-2022 kontrolü"},
	{ID: "07", Name: "Holder dağılımı ve yoğunlaşma"},
	{ID: "08", Name: "Lansman ve ilk dağılım adaleti"},
	{ID: "09", Name: "Creator ve tekrar eden aktör geçmişi"},
	{ID: "10", Name: "Fonlama grafiği ve varlık hareketleri"},
	{ID: "11", Name: "Likidite sahipliği ve çekme yetkisi"},
	{ID: "12", Name: "Satılabilirlik ve çıkış simülasyonu"},
	{ID: "13", Name: "Piyasa davranışı ve manipülasyon araştırması"},
	{ID: "14", Name: "MEV ve işlem sıralama riski"},
	{ID: "15", Name: "Cüzdan izinleri ve hesap güvenliği"},
	{ID: "16", Name: "İmza ve işlem öncesi analiz"},
	{ID: "17", Name: "İşlem politikası ve gerçek engelleme"},
	{ID: "18", Name: "EVM kontrat güvenliği"},
	{ID: "19", Name: "Solana program güvenliği"},
	{ID: "20", Name: "Güvenli test ve doğrulama laboratuvarı"},
	{ID: "21", Name: "DeFi ve ekonomik güvenlik"},
	{ID: "22", Name: "Köprü, L2 ve çapraz zincir güvenliği"},
	{ID: "23", Name: "DAO, treasury ve anahtar yönetimi"},
	{ID: "24", Name: "Claim, airdrop, NFT ve kimlik avı"},
	{ID: "25", Name: "dApp, web, API ve tedarik zinciri"},
	{ID: "26", Name: "Sürekli izleme ve erken uyarı"},
	{ID: "27", Name: "Olay müdahalesi, adli inceleme ve kurtarma"},
	{ID: "28", Name: "Tehdit istihbaratı ve analist iş akışı"},
	{ID: "29", Name: "Karar motoru ve açıklanabilir rapor"},
	{ID: "30", Name: "Kullanıcı paneli, ekip ve entegrasyonlar"},
	{ID: "31", Name: "Merkezin kendi güvenliği ve işletimi"},
	{ID: "32", Name: "AI ajanları ve Koschei Lang güvenliği"},
	{ID: "33", Name: "Kalite, kıyaslama ve üretim kabulü"},
	{ID: "34", Name: "Ek zincir ve uzmanlık adaptörleri"},
}

// Canonical source: docs/security-center/WSC-34-AREAS-170-CONTROLS-2026-10-15.md
// Keep all descriptions synchronized; CI tests guard count, structure and
// conservative verification claims, not the existence of runtime detectors.
var acceptanceRequirements = []requirementDef{
	checkDef("G01", "Hedef, zincir, sürüm, blok/slot, zaman penceresi ve kapsam kayıtlı."),
	checkDef("G02", "Collector, ayrıştırıcı ve kural gerçek kullanıcı/API akışından çağrılıyor."),
	checkDef("G03", "Bulgu, ham kaynağına ve uygulanan kural sürümüne geri izlenebiliyor."),
	checkDef("G04", "İddiaya ait zorunlu alt kontroller değerlendirilmiş; unknown durum başarı diye sunulmuyor."),
	checkDef("G05", "Yokluk, hatalı kaynak, uygulanamazlık, desteklenmeme ve kısmi kapsam birbirinden ayrılıyor."),
	checkDef("G06", "Panel, API, rapor ve paylaşılan çıktı aynı kanonik sonucu yansıtıyor."),
	checkDef("G07", "Beklenen bulgu ve normal kullanım örnekleriyle sonuç tekrar üretilebiliyor."),
	checkDef("G08", "Kayıt, izleme ve bildirim hataları görünür; engelleme iddiası varsa uygulama ayrıca kanıtlı."),
	checkDef("WSC-01.1", "Token, cüzdan, işlem, kontrat/program, protokol, havuz, NFT, URL ve depo hedeflerini ayırt et."),
	checkDef("WSC-01.2", "Zincir, ağ, adres, sürüm, sahiplik sınırı ve korunacak varlıkları kaydet."),
	checkDef("WSC-01.3", "Kritik bağımlılıkları; admin, oracle, köprü, RPC, frontend ve üçüncü taraf hizmetlerini haritala."),
	checkDef("WSC-01.4", "Taramanın sorularını, kapsam dışı alanlarını ve gerekli veri kaynaklarını başlangıçta belirle."),
	checkDef("WSC-01.5", "Aktif testler için hedef yetkisini; salt okuma, simülasyon ve canlı işlem sınırlarını ayrı tanımla."),
	checkDef("WSC-02.1", "Canlı blok/slot, hesap durumu, olay kayıtları ve geçmiş işlemleri desteklenen ağdan topla."),
	checkDef("WSC-02.2", "Arşiv erişimini, sayfalama sınırını, kesilen geçmişi ve kapsanmayan dönemi göster."),
	checkDef("WSC-02.3", "Sağlayıcı hatasını, kota aşımını ve zaman aşımını sıfır bulgudan ayır."),
	checkDef("WSC-02.4", "Finality, yeniden yapılanma, gecikme ve sağlayıcılar arası uyuşmazlığı izle."),
	checkDef("WSC-02.5", "Yeniden deneme, kaldığı yerden sürdürme, tekrar kayıt önleme ve maliyet bütçesini uygula."),
	checkDef("WSC-03.1", "Her bulguya kaynak, zaman, blok/slot ve uygun olduğunda işlem imzası bağla."),
	checkDef("WSC-03.2", "Ham yanıtı, normalleştirilmiş veriyi, kullanılan parametreleri ve içerik özetini sakla."),
	checkDef("WSC-03.3", "Collector, ayrıştırıcı ve kural sürümleriyle aynı koşullarda tekrar inceleme sağla."),
	checkDef("WSC-03.4", "Gözlem, çıkarım, harici etiket ve doğrulanmış bulguyu ayrı alanlarda tut."),
	checkDef("WSC-03.5", "İmzalı raporu doğrulama, anahtar döndürme, düzeltme geçmişi ve delil erişim denetimi sağla."),
	checkDef("WSC-04.1", "Resmî mint/kontrat, zincir, sembol, decimals ve metadata kaynağını doğrula."),
	checkDef("WSC-04.2", "Sahte token, taklit metadata, yanlış ağ ve köprülenmiş temsil ayrımını yap."),
	checkDef("WSC-04.3", "Stablecoin, memecoin, yönetim tokenı, wrapped varlık ve LP payı türlerini ayır."),
	checkDef("WSC-04.4", "Arz, dolaşım, piyasa değeri, fiyat ve likiditenin zaman ve kapsamını eşleştir."),
	checkDef("WSC-04.5", "İhraççı, itfa koşulları ve rezerv açıklamalarını tarihli dış kanıt olarak göster; zincir bakiyesiyle rezerv yeterliliği iddia etme."),
	checkDef("WSC-05.1", "Mint, burn, freeze, pause, blacklist, whitelist ve vergi değiştirme yetkilerini çıkar."),
	checkDef("WSC-05.2", "Owner, admin, rol devri, proxy, implementation ve upgrade authority ilişkilerini çöz."),
	checkDef("WSC-05.3", "Yetkileri kimin hangi imza eşiği veya zaman kilidiyle kullanabildiğini göster."),
	checkDef("WSC-05.4", "Vazgeçilmiş görünen yetkilerin alternatif roller veya yükseltme yolu üzerinden sürüp sürmediğini kontrol et."),
	checkDef("WSC-05.5", "Yetki varlığını, fiilî kullanımı ve gözlenen zararlı davranışı ayrı raporla."),
	checkDef("WSC-06.1", "Mint ve token-account uzantılarını ayrı ayrıştır; bilinmeyen uzantıyı açıkça bildir."),
	checkDef("WSC-06.2", "Permanent delegate, transfer hook, transfer fee ve bunları değiştiren yetkileri çöz."),
	checkDef("WSC-06.3", "Freeze, default account state, mint close, non-transferable ve desteklenen diğer uzantıları değerlendir."),
	checkDef("WSC-06.4", "Tanımlı fakat etkin olmayan özelliklerle etkin kontrolü; sıfır ücretle gelecekte değişebilir ücreti ayır."),
	checkDef("WSC-06.5", "Uzantıların cüzdan, DEX ve işlem simülasyonuyla uyumluluğunu kontrol et."),
	checkDef("WSC-07.1", "Token hesaplarını gerçek hesap sahibine çöz; aynı ownerın hesaplarını birleştir."),
	checkDef("WSC-07.2", "Havuz, burn, escrow, köprü, borsa/saklama ve belirsiz roller için kanıtlı sınıflandırma kullan."),
	checkDef("WSC-07.3", "Top 1/5/10, ilişkili grup payı ve hesaplama paydasını birlikte göster."),
	checkDef("WSC-07.4", "Dönemler arası yoğunlaşma, parçalama, birleşme ve büyük bakiye değişimini izle."),
	checkDef("WSC-07.5", "Bir cüzdan adresinin tek gerçek kişiyi temsil ettiğini varsaymadan kontrol riskini açıkla."),
	checkDef("WSC-08.1", "Mint oluşturma, ilk likidite ve ilk işlem zamanlarını ayrı kanıtlarla sabitle."),
	checkDef("WSC-08.2", "Tanımlı lansman aralığındaki ilk alıcıları, ilk transferleri ve arz dağılımını çıkar."),
	checkDef("WSC-08.3", "Sniper, bundle ve ilişkili alım bulgularını işlem sırası, zaman ve erişilebilir ağ verisiyle destekle."),
	checkDef("WSC-08.4", "Creator fonlaması, önceden dağıtım, vesting ve unlock şartlarını incele."),
	checkDef("WSC-08.5", "Yakın tarihli holder geçmişiyle lansman geçmişini karıştırma; arşiv eksikliğini göster."),
	checkDef("WSC-09.1", "Creator/deployer bağını oluşturma işlemi ve gerekli imzacı/hesap ilişkisiyle doğrula."),
	checkDef("WSC-09.2", "Geçmiş tokenları, kontratları, havuzları ve bunların yaşam döngülerini kaydet."),
	checkDef("WSC-09.3", "Önceki çıkış, likidite çekme veya olay bağlantılarını ayrı işlem referanslarıyla eşleştir."),
	checkDef("WSC-09.4", "Cüzdan değişimi ve teknik örüntü tekrarını olasılık ve kapsam sınırıyla göster."),
	checkDef("WSC-09.5", "Kendi veri kümesinde eşleşme olmamasını tüm zincir geçmişi temizliği olarak sunma."),
	checkDef("WSC-10.1", "Kaynak, alıcı, tutar, varlık, zaman ve işlem referanslarıyla yönlü hareket grafiği kur."),
	checkDef("WSC-10.2", "Bir ve çok atlamalı akışları, erişilen derinliği ve kesilen dalları göster."),
	checkDef("WSC-10.3", "Ortak fonlayıcı, creator-holder bağı ve ortak çıkışları bağımsız işaretlerle değerlendir."),
	checkDef("WSC-10.4", "Köprü, router ve saklama hizmeti geçişlerini desteklenen doğrulanabilir bağlantılarla izle."),
	checkDef("WSC-10.5", "Aynı fonlayıcıyı ortak kimlik; ardışık hareketi aynı fungible birimlerin aktarımı sayma."),
	checkDef("WSC-11.1", "Havuzun gerçek programını/kontratını, token çiftini ve rezervlerini doğrula."),
	checkDef("WSC-11.2", "Klasik LP tokenları ile yoğunlaştırılmış likidite pozisyonlarını ayrı çöz."),
	checkDef("WSC-11.3", "LP veya pozisyon NFT sahibini, delegate ve withdraw yetkilerini bul."),
	checkDef("WSC-11.4", "Burn, locker, kilit süresi, kilitli pay ve kilidi aşabilecek yönetici yollarını kontrol et."),
	checkDef("WSC-11.5", "Ekleme/çekme olayını instruction/çağrı izi ve uyumlu rezerv değişimiyle doğrula."),
	checkDef("WSC-12.1", "Farklı işlem büyüklükleri için mevcut rotaları ve aktif likiditeyi incele."),
	checkDef("WSC-12.2", "Alış/satış kısıtı, dinamik vergi, maksimum miktar ve adres bağımlı davranışı test et."),
	checkDef("WSC-12.3", "Fiyat etkisi, minimum alınacak miktar, ücret, slippage ve başarısızlık nedenini göster."),
	checkDef("WSC-12.4", "Farklı cüzdan/koşul kontrollerini güvenli simülasyonda yap; quote ile yürütme simülasyonunu ayır."),
	checkDef("WSC-12.5", "Zaman, blok durumu ve varsayımları kaydet; gözlenen sonuçtan gelecekte kesin satış garantisi çıkarma."),
	checkDef("WSC-13.1", "Hacim, fiyat, likidite ve işlem sayısını zaman penceresi ve kaynaklarıyla karşılaştır."),
	checkDef("WSC-13.2", "Gerçek alım/satımı transfer, mint, burn ve iç cüzdan hareketinden ayır."),
	checkDef("WSC-13.3", "Wash trading şüphesini döngü, takas, karşılık ve geri dönüş verisiyle araştır."),
	checkDef("WSC-13.4", "Koordineli alım/satım, dump, düzenli bot ritmi ve bundle bulgularını ayrı ölç."),
	checkDef("WSC-13.5", "Piyasa yapıcı, arbitraj ve normal servis davranışı için yanlış alarm kontrolleri uygula."),
	checkDef("WSC-14.1", "Swap rotası, havuz durumu, tutar, slippage ve işlem sırasını topla."),
	checkDef("WSC-14.2", "Sandwich, front-running ve back-running örüntülerini ilgili işlemlerle eşleştir."),
	checkDef("WSC-14.3", "Geçmişte gözlenen saldırıyı, mevcut maruziyeti ve öngörülen senaryoyu ayır."),
	checkDef("WSC-14.4", "Özel gönderim/bundle yolunun erişilebilirliğini ve kendi güven varsayımlarını belirt."),
	checkDef("WSC-14.5", "Yalnız ücret, Compute Budget veya Jito iziyle tamamlanmış MEV doğrulaması verme."),
	checkDef("WSC-15.1", "Token allowance, NFT operator, delegate ve kalıcı yetkileri envanterle."),
	checkDef("WSC-15.2", "Permit/Permit2, session key, akıllı hesap modülü ve EIP-7702 yetkilendirmelerini destek kapsamına al."),
	checkDef("WSC-15.3", "Sınırsız izin, süresi dolmayan yetki, değişmiş spender kodu ve yeni yönetici riskini incele."),
	checkDef("WSC-15.4", "Adres zehirleme, yanlış ağ, taklit alıcı ve kötü amaçlı bağlantı işaretlerini göster."),
	checkDef("WSC-15.5", "İzin iptalini veya anahtar değişimini önizleme ve kullanıcı yetkisiyle yürüt; tarama için seed isteme."),
	checkDef("WSC-16.1", "Mesajı, typed data'yı ve serileştirilmiş işlemi gerçek zincir formatıyla çöz."),
	checkDef("WSC-16.2", "Alıcı, tutar, çağrılan programlar, iç çağrılar ve kazanılan yetkileri anlaşılır göster."),
	checkDef("WSC-16.3", "Solana lookup table ve gerekli hesaplarını; EVM çoklu çağrı ve delegasyon etkilerini çöz."),
	checkDef("WSC-16.4", "Önce/sonra varlık, izin ve hesap durumu farkını sabit blok/slot varsayımıyla simüle et."),
	checkDef("WSC-16.5", "Domain, chain ID, nonce, süre ve imzalanacak baytların analiz edilen işlemle eşliğini doğrula."),
	checkDef("WSC-17.1", "İzin verilen program, alıcı, varlık, tutar ve yetki sınırlarını açık politika olarak tanımla."),
	checkDef("WSC-17.2", "ALLOW, REVIEW ve BLOCK önerisini uygulanan engelleme durumundan ayır."),
	checkDef("WSC-17.3", "Entegrasyonun kontrol ettiği gönderim yolunda politika ihlalini gerçekten durdur."),
	checkDef("WSC-17.4", "Alternatif rota, değişmiş işlem, replay, süresi geçmiş izin ve durum değişimi testlerini yap."),
	checkDef("WSC-17.5", "İnsan onayı, acil kapatma, politika sürümü ve uygulama sonucunu denetlenebilir kaydet."),
	checkDef("WSC-18.1", "Kaynak kodu, derleyici, bytecode ve zincirdeki proxy/implementation eşleşmesini doğrula."),
	checkDef("WSC-18.2", "Erişim kontrolü, reentrancy, dış çağrı ve delegatecall yollarını incele."),
	checkDef("WSC-18.3", "Başlatma, yükseltme, storage düzeni, imza/replay ve rastgelelik sorunlarını test et."),
	checkDef("WSC-18.4", "Yuvarlama, precision, ücretli/rebasing token uyumu, DoS ve muhasebe hatalarını araştır."),
	checkDef("WSC-18.5", "Bulgunun erişilebilir yolunu, etkilenen fonksiyonunu, etkisini ve düzeltmesini göster."),
	checkDef("WSC-19.1", "Program ID, deploy edilen binary, kaynak/IDL ve upgrade authority bağlantısını çöz."),
	checkDef("WSC-19.2", "Signer, owner, discriminator, writable ve hesap türü kontrollerini incele."),
	checkDef("WSC-19.3", "PDA seed/bump, hesap ikamesi, çift mutable hesap ve yeniden başlatma yollarını test et."),
	checkDef("WSC-19.4", "CPI hedefi, yetki aktarımı, hesap kapatma ve değişen token programı varsayımlarını kontrol et."),
	checkDef("WSC-19.5", "Arithmetik, bakiye muhasebesi ve Token-2022 etkileşimlerini yerel SVM ortamında doğrula."),
	checkDef("WSC-20.1", "Statik analiz, dinamik analiz, fuzzing ve güvenlik invariant testlerini birleştir."),
	checkDef("WSC-20.2", "Uygun kritik özellikler için varsayımları belirtilmiş sembolik/formal doğrulama uygula."),
	checkDef("WSC-20.3", "Fork/yerel SVM üzerinde yalıtılmış, tekrar üretilebilir saldırı senaryoları kullan."),
	checkDef("WSC-20.4", "Bulgu başına minimum yeniden üretim, etki kanıtı ve düzeltme sonrası regresyon kontrolü tut."),
	checkDef("WSC-20.5", "Sandbox ağ/secret sınırlarını, araç sürümlerini, test girdilerini ve çalıştırma kayıtlarını sakla."),
	checkDef("WSC-21.1", "Oracle kaynağı, tazelik, decimal, sapma, TWAP ve fallback koşullarını değerlendir."),
	checkDef("WSC-21.2", "Borçlanma, teminat, health factor, likidasyon ve bad debt senaryolarını incele."),
	checkDef("WSC-21.3", "Flash loan ile birleşen ekonomik saldırıları ve protokol invariant ihlallerini test et."),
	checkDef("WSC-21.4", "Vault pay hesabı, bağış/enflasyon etkisi, reward dağıtımı ve staking/restaking risklerini incele."),
	checkDef("WSC-21.5", "Protokoller arası bağımlılık, peg kaybı ve toplu çıkış senaryolarını açık varsayımlarla modelle."),
	checkDef("WSC-22.1", "Mesaj kaynağı, hedef zincir, doğrulayıcı seti ve güven varsayımlarını haritala."),
	checkDef("WSC-22.2", "Kanıt/imza doğrulama, finality, replay ve nonce kontrollerini incele."),
	checkDef("WSC-22.3", "Lock-mint/burn-release karşılığını ve varlık muhasebesini zincirler arasında eşleştir."),
	checkDef("WSC-22.4", "L2 sequencer, challenge süresi, forced withdrawal ve veri erişilebilirliğini değerlendir."),
	checkDef("WSC-22.5", "ZK kullanan sistemlerde devredeki proof verifier, public input ve upgrade bağlarını doğrula."),
	checkDef("WSC-23.1", "Multisig sahipleri, imza eşiği, modüller, guard ve recovery yollarını çıkar."),
	checkDef("WSC-23.2", "Teklif, oylama, quorum, timelock, veto ve yürütülecek çağrıların etkisini incele."),
	checkDef("WSC-23.3", "Admin, treasury, fee ve emergency rollerinin ayrımını ve görevden alma sürecini denetle."),
	checkDef("WSC-23.4", "Donanım cüzdanı/HSM/MPC kullanılıyorsa erişim, yedekleme ve kurtarma prosedürlerini incele."),
	checkDef("WSC-23.5", "Anahtar değişimi, rol yükseltme, büyük transfer ve yönetişim ele geçirme işaretlerine alarm üret."),
	checkDef("WSC-24.1", "Resmî alan adı, yönlendirme, taklit site ve bağlantının talep ettiği cüzdan işlemini incele."),
	checkDef("WSC-24.2", "Claim sözleşmesi/programı, dağıtım kökü, uygunluk ve imzalanan işlemin etkisini çöz."),
	checkDef("WSC-24.3", "Drainer, sahte claim, zararlı izin, NFT operator ve piyasa emri imzalarını incele."),
	checkDef("WSC-24.4", "Sahte koleksiyon, mutable metadata, sahiplik ve transfer kısıtlarını kanıtlarıyla göster."),
	checkDef("WSC-24.5", "Şüpheli içerikleri izole ortamda aç; link metnini güvenilir talimat kabul etme."),
	checkDef("WSC-25.1", "Yetkilendirme kapsamındaki frontend, backend, API, DNS ve barındırma bileşenlerini envanterle."),
	checkDef("WSC-25.2", "Kimlik doğrulama, oturum, nesne/fonksiyon erişimi, injection ve SSRF kontrollerini test et."),
	checkDef("WSC-25.3", "DNS/CDN, JavaScript ve cüzdan bağlantı bileşenlerinde yetkisiz değişikliği izle."),
	checkDef("WSC-25.4", "Bağımlılık, SBOM, secret sızıntısı, build ve dağıtım bütünlüğünü denetle."),
	checkDef("WSC-25.5", "Yetkili dinamik test, güvenlik yapılandırması ve bulgu düzeltme takibini uygula."),
	checkDef("WSC-26.1", "İzleme listelerini olay akışı, blok/slot ve hesap değişimiyle sürekli besle."),
	checkDef("WSC-26.2", "Yeni mint, authority, upgrade, LP, büyük transfer, fiyat/oracle ve izin değişimlerine alarm tanımla."),
	checkDef("WSC-26.3", "Alarm tekrarlarını birleştir; önem, etki, güven ve ilgili varlığa göre önceliklendir."),
	checkDef("WSC-26.4", "Webhook/bildirim teslimini, tekrar denemeyi ve görevliye eskalasyonu ölç."),
	checkDef("WSC-26.5", "Worker heartbeat, gecikme, olay kaybı, kesinti sonrası backfill ve yanlış alarm oranını izle."),
	checkDef("WSC-27.1", "Olayı sınıflandır; etkilenen varlıkları, zaman çizelgesini ve risk altındaki fonları belirle."),
	checkDef("WSC-27.2", "Ham kayıtları ve işlem ilişkilerini koru; inceleme sorumlusu ve iletişim planı ata."),
	checkDef("WSC-27.3", "Pause, izin iptali, anahtar değişimi ve fon taşıma seçeneklerini yetki ve etki analiziyle hazırla."),
	checkDef("WSC-27.4", "Uygun kurum/uzman koordinasyonunu, vaka kaydını ve yapılan işlemlerin makbuzlarını tut."),
	checkDef("WSC-27.5", "Kurtarma sonrası kök neden, düzeltme, yeniden tarama ve tekrarını önleme raporu üret."),
	checkDef("WSC-28.1", "Doğrulanmış olaylar, IOC'ler, teknik taktikler ve ilgili adresleri kaynaklarıyla sakla."),
	checkDef("WSC-28.2", "Harici etiketin sağlayıcısını, tarihini, doğrulama düzeyini ve geçerlilik süresini belirt."),
	checkDef("WSC-28.3", "Vaka birleştirme, analist notu, ikinci inceleme ve hatalı etiket düzeltme süreçleri sağla."),
	checkDef("WSC-28.4", "Teknik ilişkiyi kimlik, suç veya ortak kontrol iddiasından ayrı tut."),
	checkDef("WSC-28.5", "Yeni olaylardan kurallar üretirken eski örneklerde yanlış alarm ve kaçırma etkisini ölç."),
	checkDef("WSC-29.1", "Kural, eşik, sürüm ve tetikleyen kanıtla gerekçeli karar üret."),
	checkDef("WSC-29.2", "Risk şiddetini, kanıt güvenini, tarama kapsamını ve önerilen aksiyonu ayrı göster."),
	checkDef("WSC-29.3", "Aynı kanıtın birden fazla modülde görünmesini ayrı bağımsız doğrulama gibi sayma."),
	checkDef("WSC-29.4", "Tüm ekran, API ve dışa aktarımlarda aynı kanonik karar nesnesini kullan."),
	checkDef("WSC-29.5", "Eksik veriyi güvenli sayma; yeterli kanıtı olan bulguyu diğer eksiklerden bağımsız açıkça raporla."),
	checkDef("WSC-30.1", "Token/cüzdan/işlem/URL araması, inceleme dosyası, kanıt grafiği ve izleme listesi sun."),
	checkDef("WSC-30.2", "API, SDK ve webhook sözleşmelerini sürümle; kimlik, kota, idempotency ve hata durumlarını tanımla."),
	checkDef("WSC-30.3", "Kurumsal ekiplerde rol, dosya paylaşımı, atama ve denetim kaydı sağla."),
	checkDef("WSC-30.4", "Rapor dışa aktarımı, makineyle okunabilir kanıt ve geçmiş sonuç karşılaştırması sun."),
	checkDef("WSC-30.5", "Ticari hizmette abonelik/kredi kontrolünü güvenlik sonucundan ayır; ödeme hatası kanıt durumunu değiştirmesin."),
	checkDef("WSC-31.1", "MFA/SSO, en az yetki, müşteri izolasyonu, oturum güvenliği ve secret yönetimi uygula."),
	checkDef("WSC-31.2", "Veriyi aktarımda/depolamada koru; saklama, silme, yedekleme ve geri dönüş süreçlerini belirle."),
	checkDef("WSC-31.3", "Tarama girdilerine karşı SSRF, kaynak tüketimi, kötü amaçlı dosya ve kod yürütme sınırları kur."),
	checkDef("WSC-31.4", "Uygulama, veri tabanı, kuyruk, indeks, çalışan görev ve bildirim sağlık ölçümlerini izle."),
	checkDef("WSC-31.5", "Kendi pentest/bağımlılık denetimini, kurtarma tatbikatını, olay loglarını ve güvenlik bildirim kanalını işlet."),
	checkDef("WSC-32.1", "AI açıklamasını kanonik kanıt, doğrulanmış araç çıktısı ve sürümlü kurallara bağla."),
	checkDef("WSC-32.2", "URL, token metadata, kaynak kod ve belgelerdeki prompt injection'ı güvenilmeyen girdi say."),
	checkDef("WSC-32.3", "Ajanlara dar araç yetkisi, hedef sınırı, kaynak bütçesi ve süre sınırı uygula."),
	checkDef("WSC-32.4", "Lang/derleyici/runtime çıktısını sürümle, test et ve izole ortamda doğrula."),
	checkDef("WSC-32.5", "Canlı imza, yayın, müdahale ve yetki değişimini ayrı politika/onayla yönet; her araç çalışmasını kaydet."),
	checkDef("WSC-33.1", "Bilinen saldırı, normal kullanım ve zor sınır durumlarını içeren sürümlü örnek kümesi oluştur."),
	checkDef("WSC-33.2", "Yanlış pozitif, yanlış negatif, kapsama, gecikme ve tarama maliyetini ölç."),
	checkDef("WSC-33.3", "Mock test yanında gerçek sağlayıcı entegrasyonu ve üretime benzer uçtan uca test uygula."),
	checkDef("WSC-33.4", "RPC kesintisi, eksik tarihçe, bozuk ayrıştırma, kayıt hatası ve durum değişimini sınayıp sonucu kontrol et."),
	checkDef("WSC-33.5", "Yeni sürümü önceki sürümle karşılaştır; bağımsız inceleme, regresyon ve geri alma kapısı uygula."),
	checkDef("WSC-34.1", "Bitcoin/UTXO için script, PSBT, harcama koşulları, timelock ve işlem kesinliği kontrollerini tanımla."),
	checkDef("WSC-34.2", "Move tabanlı ağlarda object/resource/capability, upgrade ve ortak durum güvenliğini zincire uygun değerlendir."),
	checkDef("WSC-34.3", "Cosmos/IBC ve diğer köprülerde istemci, mesaj ve finality modeline özel doğrulama uygula."),
	checkDef("WSC-34.4", "Validator/staking, restaking, kurumsal custody, RWA veya oyun/DePIN için ayrı tehdit modeli ekle."),
	checkDef("WSC-34.5", "Her adaptöre desteklenen hedef/işlem matrisi ve kendi test kümesini ver; bir ağın sonucu diğerine taşınmasın."),
}

func acceptanceLead(area int) string {
	switch area {
	case 15, 16, 17, 23, 26, 27:
		return "Sentinel"
	case 20, 32:
		return "Koschei Lang"
	case 2, 3, 29, 30, 31, 33:
		return "shared_core"
	default:
		return "ARVIS"
	}
}

func acceptancePhase(area int) int {
	switch area {
	case 1, 2, 3, 29, 30, 31, 33:
		return 1 // provenance, evidence and canonical decisions
	case 15, 16, 17, 24, 25, 26, 27:
		return 2 // drainer, phishing, signed transaction and alerts
	case 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14:
		return 3 // token, liquidity and Solana risk
	case 18, 19, 20, 21, 22, 23:
		return 4 // audits, DeFi, bridge and DAO custody
	default:
		return 5 // intelligence, Lang and further adapters
	}
}

// CurrentAcceptanceCatalog is an evidence register, not a runtime scan.
// Every control starts unverified until a distinct acceptance procedure has
// been executed, reviewed and attached; no capabilities are inferred from the
// existence of a source file or passing compilation tests.
func CurrentAcceptanceCatalog() AcceptanceCatalog {
	areas := make([]AcceptanceArea, len(acceptanceAreas))
	copy(areas, acceptanceAreas)
	areaTitles := make(map[string]struct{}, len(areas))
	for i := range areas {
		n, err := strconv.Atoi(areas[i].ID)
		if err != nil {
			continue
		}
		areas[i].Lead = acceptanceLead(n)
		areas[i].Phase = acceptancePhase(n)
		areas[i].CheckCount = 5
		areaTitles[areas[i].ID] = struct{}{}
	}
	checks := make([]AcceptanceCheck, 0, len(acceptanceRequirements))
	gates, controls := 0, 0
	for _, definition := range acceptanceRequirements {
		c := AcceptanceCheck{
			ID: definition.id,
			Description: definition.description,
			Applicability: AcceptanceNotAssessed,
			Verification: AcceptanceUnverified,
			EvidenceRefs: []string{},
		}
		if strings.HasPrefix(definition.id, "G") {
			gates++
			c.Lead = "shared_core"
			c.Phase = 1
		} else {
			controls++
			if len(definition.id) >= 6 {
				c.AreaID = definition.id[4:6]
				n, err := strconv.Atoi(c.AreaID)
				if err == nil {
					c.Lead = acceptanceLead(n)
					c.Phase = acceptancePhase(n)
				}
				if _, ok := areaTitles[c.AreaID]; !ok {
					c.Lead = "catalog_area_unresolved"
				}
			}
		}
		checks = append(checks, c)
	}
	return AcceptanceCatalog{
		SchemaVersion: AcceptanceCatalogSchemaVersion,
		SourceDocument: "docs/security-center/WSC-34-AREAS-170-CONTROLS-2026-10-15.md",
		ScopeNote: fmt.Sprintf("%d WSC controls and %d common gates are requirements only; none is certified by this catalog. Existing repository capabilities and deployed enforcement require independent evidence.", controls, gates),
		AreaCount: len(areas),
		ControlCount: controls,
		CommonGateCount: gates,
		VerifiedCount: 0,
		Areas: areas,
		Checks: checks,
	}
}
