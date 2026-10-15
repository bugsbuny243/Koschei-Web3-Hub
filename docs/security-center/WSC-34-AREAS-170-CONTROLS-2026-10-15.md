# Koschei — Web3 Güvenlik Merkezi Gereksinim ve Kabul Listesi

Tarih: 15 Ekim 2026
Kapsam: 34 yetenek alanı, 170 alan kontrolü ve 8 ortak kabul koşulu.

Bu belge Koschei için önerilen kapsamlı bir ürün ve mühendislik gereksinim kataloğudur. Mevcut kodun denetim sonucu veya bugün çalıştığı onaylanmış özelliklerin listesi değildir. İşaretlenmemiş kutular eksiklik hükmü taşımaz; kabul kanıtı henüz bu belgeye işlenmemiştir.

Çekirdek alanlar tüm merkez için geçerlidir. EVM, Solana, köprü, custody veya başka uzmanlık alanı ilgili destek ilan edildiğinde uygulanır. Bütün zincirlerin ilk sürümde desteklenmesi gerekmez; destek beyanı gerçek adaptör, veri ve test kapsamıyla aynı olmalıdır.

Kaynaklar teknik kontrol ailelerini destekler. Bu gruplama ve ürün kabul koşulları bir mühendislik sentezidir; tek bir resmî standardın bütün maddeleri veya bir sertifikasyon iddiası değildir.

## Sonuç durumlarının ayrılması

| Boyut | Örnek durumlar | Anlamı |
|---|---|---|
| Çalıştırma | not_run, running, completed, partial, failed, not_applicable, unsupported | İşin çalışması ve uygulanabilirliği |
| Kanıt yeterliliği | sufficient, partial, stale, unavailable, conflicting | İlgili iddiayı destekleyen verinin durumu |
| Bulgu | none_observed, suspected, verified | İncelenen pencerede ne gözlendiği |
| Risk şiddeti | informational, low, medium, high, critical | Bulguyla ilişkili etki ve önem |
| Önerilen aksiyon | allow, review, block | Belirli politikaya göre öneri |
| Uygulama | not_connected, shadow, enforced, failed | Kontrolün gerçek gönderim yolunda uygulanması |
| Rapor bütünlüğü | unsigned, signature_valid, signature_invalid | İmzalanan içerikle eşleşme; içeriğin maddi doğruluğu ayrı konudur |

VERIFIED belirli bir iddianın gerekli kanıtlarla doğrulandığını ifade etmelidir. Bir uzantının veya yetkinin varlığını doğrulamak, hedefin güvenli olduğunu doğrulamaz. Tamamlanmış fakat bulgu üretmemiş bir inceleme de ayrı ve değerli bir sonuçtur. Diğer kollar eksik olsa bile yeterli kanıtı bulunan ciddi bir bulgu gösterilir; eksik veriler gizlenmez.

## Ortak kabul koşulları

- [ ] G01: Hedef, zincir, sürüm, blok/slot, zaman penceresi ve kapsam kayıtlı.
- [ ] G02: Collector, ayrıştırıcı ve kural gerçek kullanıcı/API akışından çağrılıyor.
- [ ] G03: Bulgu, ham kaynağına ve uygulanan kural sürümüne geri izlenebiliyor.
- [ ] G04: İddiaya ait zorunlu alt kontroller değerlendirilmiş; unknown durum başarı diye sunulmuyor.
- [ ] G05: Yokluk, hatalı kaynak, uygulanamazlık, desteklenmeme ve kısmi kapsam birbirinden ayrılıyor.
- [ ] G06: Panel, API, rapor ve paylaşılan çıktı aynı kanonik sonucu yansıtıyor.
- [ ] G07: Beklenen bulgu ve normal kullanım örnekleriyle sonuç tekrar üretilebiliyor.
- [ ] G08: Kayıt, izleme ve bildirim hataları görünür; engelleme iddiası varsa uygulama ayrıca kanıtlı.

## 01. Hedef envanteri ve tehdit modeli

Kapsam: Çekirdek.

- [ ] WSC-01.1: Token, cüzdan, işlem, kontrat/program, protokol, havuz, NFT, URL ve depo hedeflerini ayırt et.
- [ ] WSC-01.2: Zincir, ağ, adres, sürüm, sahiplik sınırı ve korunacak varlıkları kaydet.
- [ ] WSC-01.3: Kritik bağımlılıkları; admin, oracle, köprü, RPC, frontend ve üçüncü taraf hizmetlerini haritala.
- [ ] WSC-01.4: Taramanın sorularını, kapsam dışı alanlarını ve gerekli veri kaynaklarını başlangıçta belirle.
- [ ] WSC-01.5: Aktif testler için hedef yetkisini; salt okuma, simülasyon ve canlı işlem sınırlarını ayrı tanımla.

Kabul kanıtı: Hedefin türü ve güven sınırları belli; her kontrolün uygulanabilirlik nedeni kayıtlı.

Teknik dayanak: S1.

## 02. Zincir verisi, RPC ve indeksleme

Kapsam: Çekirdek.

- [ ] WSC-02.1: Canlı blok/slot, hesap durumu, olay kayıtları ve geçmiş işlemleri desteklenen ağdan topla.
- [ ] WSC-02.2: Arşiv erişimini, sayfalama sınırını, kesilen geçmişi ve kapsanmayan dönemi göster.
- [ ] WSC-02.3: Sağlayıcı hatasını, kota aşımını ve zaman aşımını sıfır bulgudan ayır.
- [ ] WSC-02.4: Finality, yeniden yapılanma, gecikme ve sağlayıcılar arası uyuşmazlığı izle.
- [ ] WSC-02.5: Yeniden deneme, kaldığı yerden sürdürme, tekrar kayıt önleme ve maliyet bütçesini uygula.

Kabul kanıtı: Bir olay yeniden oynatıldığında kaybolmuyor veya çift sayılmıyor; geçmiş boşlukları görünür.

Teknik dayanak: S2.

## 03. Kanıt bütünlüğü ve izlenebilirlik

Kapsam: Çekirdek.

- [ ] WSC-03.1: Her bulguya kaynak, zaman, blok/slot ve uygun olduğunda işlem imzası bağla.
- [ ] WSC-03.2: Ham yanıtı, normalleştirilmiş veriyi, kullanılan parametreleri ve içerik özetini sakla.
- [ ] WSC-03.3: Collector, ayrıştırıcı ve kural sürümleriyle aynı koşullarda tekrar inceleme sağla.
- [ ] WSC-03.4: Gözlem, çıkarım, harici etiket ve doğrulanmış bulguyu ayrı alanlarda tut.
- [ ] WSC-03.5: İmzalı raporu doğrulama, anahtar döndürme, düzeltme geçmişi ve delil erişim denetimi sağla.

Kabul kanıtı: Bir bulgudan ham kayda gidiliyor; rapor değişikliği ve imza geçerliliği denetlenebiliyor.

Teknik dayanak: S3.

## 04. Token kimliği ve ekonomik bağlam

Kapsam: Token desteğinde.

- [ ] WSC-04.1: Resmî mint/kontrat, zincir, sembol, decimals ve metadata kaynağını doğrula.
- [ ] WSC-04.2: Sahte token, taklit metadata, yanlış ağ ve köprülenmiş temsil ayrımını yap.
- [ ] WSC-04.3: Stablecoin, memecoin, yönetim tokenı, wrapped varlık ve LP payı türlerini ayır.
- [ ] WSC-04.4: Arz, dolaşım, piyasa değeri, fiyat ve likiditenin zaman ve kapsamını eşleştir.
- [ ] WSC-04.5: İhraççı, itfa koşulları ve rezerv açıklamalarını tarihli dış kanıt olarak göster; zincir bakiyesiyle rezerv yeterliliği iddia etme.

Kabul kanıtı: Aynı sembollü iki varlık karışmıyor; ekonomik karşılaştırmanın payı ve paydası açık.

## 05. Yönetim yetkileri ve kontrol haritası

Kapsam: Token/program desteğinde.

- [ ] WSC-05.1: Mint, burn, freeze, pause, blacklist, whitelist ve vergi değiştirme yetkilerini çıkar.
- [ ] WSC-05.2: Owner, admin, rol devri, proxy, implementation ve upgrade authority ilişkilerini çöz.
- [ ] WSC-05.3: Yetkileri kimin hangi imza eşiği veya zaman kilidiyle kullanabildiğini göster.
- [ ] WSC-05.4: Vazgeçilmiş görünen yetkilerin alternatif roller veya yükseltme yolu üzerinden sürüp sürmediğini kontrol et.
- [ ] WSC-05.5: Yetki varlığını, fiilî kullanımı ve gözlenen zararlı davranışı ayrı raporla.

Kabul kanıtı: Her kritik yetki için yetkili adres, uygulanabilir işlem ve zincir kanıtı mevcut.

Teknik dayanak: S4.

## 06. Solana Token-2022 kontrolü

Kapsam: Solana token desteğinde.

- [ ] WSC-06.1: Mint ve token-account uzantılarını ayrı ayrıştır; bilinmeyen uzantıyı açıkça bildir.
- [ ] WSC-06.2: Permanent delegate, transfer hook, transfer fee ve bunları değiştiren yetkileri çöz.
- [ ] WSC-06.3: Freeze, default account state, mint close, non-transferable ve desteklenen diğer uzantıları değerlendir.
- [ ] WSC-06.4: Tanımlı fakat etkin olmayan özelliklerle etkin kontrolü; sıfır ücretle gelecekte değişebilir ücreti ayır.
- [ ] WSC-06.5: Uzantıların cüzdan, DEX ve işlem simülasyonuyla uyumluluğunu kontrol et.

Kabul kanıtı: Uzantı adı, etkin durum, parametre ve yetki adresi kanıtlı; desteklenmeyen durum eksik işaretli.

Teknik dayanak: S5.

## 07. Holder dağılımı ve yoğunlaşma

Kapsam: Token desteğinde.

- [ ] WSC-07.1: Token hesaplarını gerçek hesap sahibine çöz; aynı ownerın hesaplarını birleştir.
- [ ] WSC-07.2: Havuz, burn, escrow, köprü, borsa/saklama ve belirsiz roller için kanıtlı sınıflandırma kullan.
- [ ] WSC-07.3: Top 1/5/10, ilişkili grup payı ve hesaplama paydasını birlikte göster.
- [ ] WSC-07.4: Dönemler arası yoğunlaşma, parçalama, birleşme ve büyük bakiye değişimini izle.
- [ ] WSC-07.5: Bir cüzdan adresinin tek gerçek kişiyi temsil ettiğini varsaymadan kontrol riskini açıkla.

Kabul kanıtı: Ham hesap listesi, owner eşlemesi, dışlama gerekçeleri ve yoğunlaşma hesabı yeniden üretilebilir.

## 08. Lansman ve ilk dağılım adaleti

Kapsam: Lansman analizi sunuluyorsa.

- [ ] WSC-08.1: Mint oluşturma, ilk likidite ve ilk işlem zamanlarını ayrı kanıtlarla sabitle.
- [ ] WSC-08.2: Tanımlı lansman aralığındaki ilk alıcıları, ilk transferleri ve arz dağılımını çıkar.
- [ ] WSC-08.3: Sniper, bundle ve ilişkili alım bulgularını işlem sırası, zaman ve erişilebilir ağ verisiyle destekle.
- [ ] WSC-08.4: Creator fonlaması, önceden dağıtım, vesting ve unlock şartlarını incele.
- [ ] WSC-08.5: Yakın tarihli holder geçmişiyle lansman geçmişini karıştırma; arşiv eksikliğini göster.

Kabul kanıtı: Lansman penceresi gerçekten kapsanmış; her sınıflandırmanın zaman ve işlem dayanağı var.

## 09. Creator ve tekrar eden aktör geçmişi

Kapsam: Aktör araştırması sunuluyorsa.

- [ ] WSC-09.1: Creator/deployer bağını oluşturma işlemi ve gerekli imzacı/hesap ilişkisiyle doğrula.
- [ ] WSC-09.2: Geçmiş tokenları, kontratları, havuzları ve bunların yaşam döngülerini kaydet.
- [ ] WSC-09.3: Önceki çıkış, likidite çekme veya olay bağlantılarını ayrı işlem referanslarıyla eşleştir.
- [ ] WSC-09.4: Cüzdan değişimi ve teknik örüntü tekrarını olasılık ve kapsam sınırıyla göster.
- [ ] WSC-09.5: Kendi veri kümesinde eşleşme olmamasını tüm zincir geçmişi temizliği olarak sunma.

Kabul kanıtı: Her geçmiş ilişki ayrı hedefe ve olaya bağlı; yalnız benzer isim üzerinden eşleşme yok.

## 10. Fonlama grafiği ve varlık hareketleri

Kapsam: Aktör/cüzdan araştırmasında.

- [ ] WSC-10.1: Kaynak, alıcı, tutar, varlık, zaman ve işlem referanslarıyla yönlü hareket grafiği kur.
- [ ] WSC-10.2: Bir ve çok atlamalı akışları, erişilen derinliği ve kesilen dalları göster.
- [ ] WSC-10.3: Ortak fonlayıcı, creator-holder bağı ve ortak çıkışları bağımsız işaretlerle değerlendir.
- [ ] WSC-10.4: Köprü, router ve saklama hizmeti geçişlerini desteklenen doğrulanabilir bağlantılarla izle.
- [ ] WSC-10.5: Aynı fonlayıcıyı ortak kimlik; ardışık hareketi aynı fungible birimlerin aktarımı sayma.

Kabul kanıtı: Grafikte her kenar kanıtlı; hizmet cüzdanları ve belirsiz bağlantılar ayrı görülebiliyor.

Teknik dayanak: S3.

## 11. Likidite sahipliği ve çekme yetkisi

Kapsam: DEX/pool desteğinde.

- [ ] WSC-11.1: Havuzun gerçek programını/kontratını, token çiftini ve rezervlerini doğrula.
- [ ] WSC-11.2: Klasik LP tokenları ile yoğunlaştırılmış likidite pozisyonlarını ayrı çöz.
- [ ] WSC-11.3: LP veya pozisyon NFT sahibini, delegate ve withdraw yetkilerini bul.
- [ ] WSC-11.4: Burn, locker, kilit süresi, kilitli pay ve kilidi aşabilecek yönetici yollarını kontrol et.
- [ ] WSC-11.5: Ekleme/çekme olayını instruction/çağrı izi ve uyumlu rezerv değişimiyle doğrula.

Kabul kanıtı: Likiditeyi kim, ne kadar ve hangi koşulla çekebilir sorusu yanıtlı veya açıkça eksik.

Teknik dayanak: S6.

## 12. Satılabilirlik ve çıkış simülasyonu

Kapsam: Token işlem riski sunuluyorsa.

- [ ] WSC-12.1: Farklı işlem büyüklükleri için mevcut rotaları ve aktif likiditeyi incele.
- [ ] WSC-12.2: Alış/satış kısıtı, dinamik vergi, maksimum miktar ve adres bağımlı davranışı test et.
- [ ] WSC-12.3: Fiyat etkisi, minimum alınacak miktar, ücret, slippage ve başarısızlık nedenini göster.
- [ ] WSC-12.4: Farklı cüzdan/koşul kontrollerini güvenli simülasyonda yap; quote ile yürütme simülasyonunu ayır.
- [ ] WSC-12.5: Zaman, blok durumu ve varsayımları kaydet; gözlenen sonuçtan gelecekte kesin satış garantisi çıkarma.

Kabul kanıtı: Tutar ve koşulları belli simülasyon sonucu var; toplam TVL satış derinliği yerine kullanılmıyor.

Teknik dayanak: S6, S7.

## 13. Piyasa davranışı ve manipülasyon araştırması

Kapsam: Piyasa istihbaratı sunuluyorsa.

- [ ] WSC-13.1: Hacim, fiyat, likidite ve işlem sayısını zaman penceresi ve kaynaklarıyla karşılaştır.
- [ ] WSC-13.2: Gerçek alım/satımı transfer, mint, burn ve iç cüzdan hareketinden ayır.
- [ ] WSC-13.3: Wash trading şüphesini döngü, takas, karşılık ve geri dönüş verisiyle araştır.
- [ ] WSC-13.4: Koordineli alım/satım, dump, düzenli bot ritmi ve bundle bulgularını ayrı ölç.
- [ ] WSC-13.5: Piyasa yapıcı, arbitraj ve normal servis davranışı için yanlış alarm kontrolleri uygula.

Kabul kanıtı: Manipülasyon iddiasının işlem ve ekonomik ilişki kanıtı var; tek oran tek başına kesin suçlama üretmiyor.

## 14. MEV ve işlem sıralama riski

Kapsam: MEV ürünü sunuluyorsa.

- [ ] WSC-14.1: Swap rotası, havuz durumu, tutar, slippage ve işlem sırasını topla.
- [ ] WSC-14.2: Sandwich, front-running ve back-running örüntülerini ilgili işlemlerle eşleştir.
- [ ] WSC-14.3: Geçmişte gözlenen saldırıyı, mevcut maruziyeti ve öngörülen senaryoyu ayır.
- [ ] WSC-14.4: Özel gönderim/bundle yolunun erişilebilirliğini ve kendi güven varsayımlarını belirt.
- [ ] WSC-14.5: Yalnız ücret, Compute Budget veya Jito iziyle tamamlanmış MEV doğrulaması verme.

Kabul kanıtı: İddia edilen MEV bulgusu işlem sırası ve ekonomik etkiyle destekli; eksik sıra verisi açık.

Teknik dayanak: S2.

## 15. Cüzdan izinleri ve hesap güvenliği

Kapsam: Cüzdan korumasında.

- [ ] WSC-15.1: Token allowance, NFT operator, delegate ve kalıcı yetkileri envanterle.
- [ ] WSC-15.2: Permit/Permit2, session key, akıllı hesap modülü ve EIP-7702 yetkilendirmelerini destek kapsamına al.
- [ ] WSC-15.3: Sınırsız izin, süresi dolmayan yetki, değişmiş spender kodu ve yeni yönetici riskini incele.
- [ ] WSC-15.4: Adres zehirleme, yanlış ağ, taklit alıcı ve kötü amaçlı bağlantı işaretlerini göster.
- [ ] WSC-15.5: İzin iptalini veya anahtar değişimini önizleme ve kullanıcı yetkisiyle yürüt; tarama için seed isteme.

Kabul kanıtı: Kimin hangi varlığa erişebildiği gösteriliyor; düzeltme sonrası yetki durumu yeniden okunuyor.

Teknik dayanak: S8, S9.

## 16. İmza ve işlem öncesi analiz

Kapsam: İşlem korumasında.

- [ ] WSC-16.1: Mesajı, typed data'yı ve serileştirilmiş işlemi gerçek zincir formatıyla çöz.
- [ ] WSC-16.2: Alıcı, tutar, çağrılan programlar, iç çağrılar ve kazanılan yetkileri anlaşılır göster.
- [ ] WSC-16.3: Solana lookup table ve gerekli hesaplarını; EVM çoklu çağrı ve delegasyon etkilerini çöz.
- [ ] WSC-16.4: Önce/sonra varlık, izin ve hesap durumu farkını sabit blok/slot varsayımıyla simüle et.
- [ ] WSC-16.5: Domain, chain ID, nonce, süre ve imzalanacak baytların analiz edilen işlemle eşliğini doğrula.

Kabul kanıtı: Kullanıcının onayladığı işlemle analiz edilen işlem aynı; simülasyon eksikleri görünür.

Teknik dayanak: S7, S8, S9.

## 17. İşlem politikası ve gerçek engelleme

Kapsam: Enforcement/Guard sunuluyorsa.

- [ ] WSC-17.1: İzin verilen program, alıcı, varlık, tutar ve yetki sınırlarını açık politika olarak tanımla.
- [ ] WSC-17.2: ALLOW, REVIEW ve BLOCK önerisini uygulanan engelleme durumundan ayır.
- [ ] WSC-17.3: Entegrasyonun kontrol ettiği gönderim yolunda politika ihlalini gerçekten durdur.
- [ ] WSC-17.4: Alternatif rota, değişmiş işlem, replay, süresi geçmiş izin ve durum değişimi testlerini yap.
- [ ] WSC-17.5: İnsan onayı, acil kapatma, politika sürümü ve uygulama sonucunu denetlenebilir kaydet.

Kabul kanıtı: Yetkili testte yasak işlem gönderilmiyor; kontrol dışı yollar ve kalan yetkiler açık.

## 18. EVM kontrat güvenliği

Kapsam: EVM denetimi sunuluyorsa.

- [ ] WSC-18.1: Kaynak kodu, derleyici, bytecode ve zincirdeki proxy/implementation eşleşmesini doğrula.
- [ ] WSC-18.2: Erişim kontrolü, reentrancy, dış çağrı ve delegatecall yollarını incele.
- [ ] WSC-18.3: Başlatma, yükseltme, storage düzeni, imza/replay ve rastgelelik sorunlarını test et.
- [ ] WSC-18.4: Yuvarlama, precision, ücretli/rebasing token uyumu, DoS ve muhasebe hatalarını araştır.
- [ ] WSC-18.5: Bulgunun erişilebilir yolunu, etkilenen fonksiyonunu, etkisini ve düzeltmesini göster.

Kabul kanıtı: Sürüme bağlı bulgu, somut kod/iz ve doğrulanabilir testle destekli; kaynak doğrulama güvenlik onayı sayılmıyor.

Teknik dayanak: S1, S2, S10.

## 19. Solana program güvenliği

Kapsam: Solana program denetiminde.

- [ ] WSC-19.1: Program ID, deploy edilen binary, kaynak/IDL ve upgrade authority bağlantısını çöz.
- [ ] WSC-19.2: Signer, owner, discriminator, writable ve hesap türü kontrollerini incele.
- [ ] WSC-19.3: PDA seed/bump, hesap ikamesi, çift mutable hesap ve yeniden başlatma yollarını test et.
- [ ] WSC-19.4: CPI hedefi, yetki aktarımı, hesap kapatma ve değişen token programı varsayımlarını kontrol et.
- [ ] WSC-19.5: Arithmetik, bakiye muhasebesi ve Token-2022 etkileşimlerini yerel SVM ortamında doğrula.

Kabul kanıtı: Token hesabını tanımanın ötesinde gerçek program kodu/izleri incelenmiş; eksik artefaktlar belirtilmiş.

Teknik dayanak: S11, S12.

## 20. Güvenli test ve doğrulama laboratuvarı

Kapsam: Kod denetimi sunuluyorsa.

- [ ] WSC-20.1: Statik analiz, dinamik analiz, fuzzing ve güvenlik invariant testlerini birleştir.
- [ ] WSC-20.2: Uygun kritik özellikler için varsayımları belirtilmiş sembolik/formal doğrulama uygula.
- [ ] WSC-20.3: Fork/yerel SVM üzerinde yalıtılmış, tekrar üretilebilir saldırı senaryoları kullan.
- [ ] WSC-20.4: Bulgu başına minimum yeniden üretim, etki kanıtı ve düzeltme sonrası regresyon kontrolü tut.
- [ ] WSC-20.5: Sandbox ağ/secret sınırlarını, araç sürümlerini, test girdilerini ve çalıştırma kayıtlarını sakla.

Kabul kanıtı: Başka bir incelemeci aynı koşullarla sonucu tekrar üretebiliyor; üretimde izinsiz deneme gerekmiyor.

Teknik dayanak: S2.

## 21. DeFi ve ekonomik güvenlik

Kapsam: İlgili protokoller destekleniyorsa.

- [ ] WSC-21.1: Oracle kaynağı, tazelik, decimal, sapma, TWAP ve fallback koşullarını değerlendir.
- [ ] WSC-21.2: Borçlanma, teminat, health factor, likidasyon ve bad debt senaryolarını incele.
- [ ] WSC-21.3: Flash loan ile birleşen ekonomik saldırıları ve protokol invariant ihlallerini test et.
- [ ] WSC-21.4: Vault pay hesabı, bağış/enflasyon etkisi, reward dağıtımı ve staking/restaking risklerini incele.
- [ ] WSC-21.5: Protokoller arası bağımlılık, peg kaybı ve toplu çıkış senaryolarını açık varsayımlarla modelle.

Kabul kanıtı: Senaryo etkisi para/varlık akışı ve protokol koşullarıyla açıklanmış; ekonomik risk kod bulgusundan ayrılmış.

Teknik dayanak: S1, S2.

## 22. Köprü, L2 ve çapraz zincir güvenliği

Kapsam: Bu sistemler destekleniyorsa.

- [ ] WSC-22.1: Mesaj kaynağı, hedef zincir, doğrulayıcı seti ve güven varsayımlarını haritala.
- [ ] WSC-22.2: Kanıt/imza doğrulama, finality, replay ve nonce kontrollerini incele.
- [ ] WSC-22.3: Lock-mint/burn-release karşılığını ve varlık muhasebesini zincirler arasında eşleştir.
- [ ] WSC-22.4: L2 sequencer, challenge süresi, forced withdrawal ve veri erişilebilirliğini değerlendir.
- [ ] WSC-22.5: ZK kullanan sistemlerde devredeki proof verifier, public input ve upgrade bağlarını doğrula.

Kabul kanıtı: Her çapraz zincir iddiası iki tarafın kanıtına bağlı; özel adaptör yoksa desteklenmiyor olarak işaretli.

Teknik dayanak: S1.

## 23. DAO, treasury ve anahtar yönetimi

Kapsam: Kurumsal/DAO korumasında.

- [ ] WSC-23.1: Multisig sahipleri, imza eşiği, modüller, guard ve recovery yollarını çıkar.
- [ ] WSC-23.2: Teklif, oylama, quorum, timelock, veto ve yürütülecek çağrıların etkisini incele.
- [ ] WSC-23.3: Admin, treasury, fee ve emergency rollerinin ayrımını ve görevden alma sürecini denetle.
- [ ] WSC-23.4: Donanım cüzdanı/HSM/MPC kullanılıyorsa erişim, yedekleme ve kurtarma prosedürlerini incele.
- [ ] WSC-23.5: Anahtar değişimi, rol yükseltme, büyük transfer ve yönetişim ele geçirme işaretlerine alarm üret.

Kabul kanıtı: Kimlerin hangi eşikle fon veya kod üzerinde etkili olduğu kanıtlı; acil prosedür tatbik edilmiş.

Teknik dayanak: S4.

## 24. Claim, airdrop, NFT ve kimlik avı

Kapsam: Kullanıcı güvenliğinde.

- [ ] WSC-24.1: Resmî alan adı, yönlendirme, taklit site ve bağlantının talep ettiği cüzdan işlemini incele.
- [ ] WSC-24.2: Claim sözleşmesi/programı, dağıtım kökü, uygunluk ve imzalanan işlemin etkisini çöz.
- [ ] WSC-24.3: Drainer, sahte claim, zararlı izin, NFT operator ve piyasa emri imzalarını incele.
- [ ] WSC-24.4: Sahte koleksiyon, mutable metadata, sahiplik ve transfer kısıtlarını kanıtlarıyla göster.
- [ ] WSC-24.5: Şüpheli içerikleri izole ortamda aç; link metnini güvenilir talimat kabul etme.

Kabul kanıtı: URL taraması ile zincir işlemi analizi birlikte; yalnız token adresiyle claim güvenliği sonucu üretilmiyor.

Teknik dayanak: S8, S13.

## 25. dApp, web, API ve tedarik zinciri

Kapsam: İlgili projeye denetim veriliyorsa.

- [ ] WSC-25.1: Yetkilendirme kapsamındaki frontend, backend, API, DNS ve barındırma bileşenlerini envanterle.
- [ ] WSC-25.2: Kimlik doğrulama, oturum, nesne/fonksiyon erişimi, injection ve SSRF kontrollerini test et.
- [ ] WSC-25.3: DNS/CDN, JavaScript ve cüzdan bağlantı bileşenlerinde yetkisiz değişikliği izle.
- [ ] WSC-25.4: Bağımlılık, SBOM, secret sızıntısı, build ve dağıtım bütünlüğünü denetle.
- [ ] WSC-25.5: Yetkili dinamik test, güvenlik yapılandırması ve bulgu düzeltme takibini uygula.

Kabul kanıtı: Saldırganın web üzerinden cüzdana veya yöneticiye erişebileceği yollar değerlendirilmiş.

Teknik dayanak: S13, S14.

## 26. Sürekli izleme ve erken uyarı

Kapsam: Operasyon merkezi çekirdeği.

- [ ] WSC-26.1: İzleme listelerini olay akışı, blok/slot ve hesap değişimiyle sürekli besle.
- [ ] WSC-26.2: Yeni mint, authority, upgrade, LP, büyük transfer, fiyat/oracle ve izin değişimlerine alarm tanımla.
- [ ] WSC-26.3: Alarm tekrarlarını birleştir; önem, etki, güven ve ilgili varlığa göre önceliklendir.
- [ ] WSC-26.4: Webhook/bildirim teslimini, tekrar denemeyi ve görevliye eskalasyonu ölç.
- [ ] WSC-26.5: Worker heartbeat, gecikme, olay kaybı, kesinti sonrası backfill ve yanlış alarm oranını izle.

Kabul kanıtı: Kontrollü olay uçtan uca alarm üretiyor; sessiz duran worker sağlıklı görünmüyor.

Teknik dayanak: S3.

## 27. Olay müdahalesi, adli inceleme ve kurtarma

Kapsam: Operasyon merkezi çekirdeği.

- [ ] WSC-27.1: Olayı sınıflandır; etkilenen varlıkları, zaman çizelgesini ve risk altındaki fonları belirle.
- [ ] WSC-27.2: Ham kayıtları ve işlem ilişkilerini koru; inceleme sorumlusu ve iletişim planı ata.
- [ ] WSC-27.3: Pause, izin iptali, anahtar değişimi ve fon taşıma seçeneklerini yetki ve etki analiziyle hazırla.
- [ ] WSC-27.4: Uygun kurum/uzman koordinasyonunu, vaka kaydını ve yapılan işlemlerin makbuzlarını tut.
- [ ] WSC-27.5: Kurtarma sonrası kök neden, düzeltme, yeniden tarama ve tekrarını önleme raporu üret.

Kabul kanıtı: Müdahale planı tatbik edilmiş; dış iletişim ve zincir işlemleri açık yetkiyle yürütülüyor.

Teknik dayanak: S3.

## 28. Tehdit istihbaratı ve analist iş akışı

Kapsam: Operasyon merkezi çekirdeği.

- [ ] WSC-28.1: Doğrulanmış olaylar, IOC'ler, teknik taktikler ve ilgili adresleri kaynaklarıyla sakla.
- [ ] WSC-28.2: Harici etiketin sağlayıcısını, tarihini, doğrulama düzeyini ve geçerlilik süresini belirt.
- [ ] WSC-28.3: Vaka birleştirme, analist notu, ikinci inceleme ve hatalı etiket düzeltme süreçleri sağla.
- [ ] WSC-28.4: Teknik ilişkiyi kimlik, suç veya ortak kontrol iddiasından ayrı tut.
- [ ] WSC-28.5: Yeni olaylardan kurallar üretirken eski örneklerde yanlış alarm ve kaçırma etkisini ölç.

Kabul kanıtı: Her istihbarat kaydı kaynağına geri izleniyor ve itiraz/düzeltme geçmişi taşıyor.

Teknik dayanak: S3.

## 29. Karar motoru ve açıklanabilir rapor

Kapsam: Çekirdek.

- [ ] WSC-29.1: Kural, eşik, sürüm ve tetikleyen kanıtla gerekçeli karar üret.
- [ ] WSC-29.2: Risk şiddetini, kanıt güvenini, tarama kapsamını ve önerilen aksiyonu ayrı göster.
- [ ] WSC-29.3: Aynı kanıtın birden fazla modülde görünmesini ayrı bağımsız doğrulama gibi sayma.
- [ ] WSC-29.4: Tüm ekran, API ve dışa aktarımlarda aynı kanonik karar nesnesini kullan.
- [ ] WSC-29.5: Eksik veriyi güvenli sayma; yeterli kanıtı olan bulguyu diğer eksiklerden bağımsız açıkça raporla.

Kabul kanıtı: Tek hedef ve veri sürümü için çelişen karar yok; her sonuçtan gerekçeye gidilebiliyor.

## 30. Kullanıcı paneli, ekip ve entegrasyonlar

Kapsam: Hizmet sunumunda.

- [ ] WSC-30.1: Token/cüzdan/işlem/URL araması, inceleme dosyası, kanıt grafiği ve izleme listesi sun.
- [ ] WSC-30.2: API, SDK ve webhook sözleşmelerini sürümle; kimlik, kota, idempotency ve hata durumlarını tanımla.
- [ ] WSC-30.3: Kurumsal ekiplerde rol, dosya paylaşımı, atama ve denetim kaydı sağla.
- [ ] WSC-30.4: Rapor dışa aktarımı, makineyle okunabilir kanıt ve geçmiş sonuç karşılaştırması sun.
- [ ] WSC-30.5: Ticari hizmette abonelik/kredi kontrolünü güvenlik sonucundan ayır; ödeme hatası kanıt durumunu değiştirmesin.

Kabul kanıtı: Müşteri yetkili olduğu veriyi alıyor; aynı inceleme panel ve API'de tutarlı.

Teknik dayanak: S14.

## 31. Merkezin kendi güvenliği ve işletimi

Kapsam: Çekirdek.

- [ ] WSC-31.1: MFA/SSO, en az yetki, müşteri izolasyonu, oturum güvenliği ve secret yönetimi uygula.
- [ ] WSC-31.2: Veriyi aktarımda/depolamada koru; saklama, silme, yedekleme ve geri dönüş süreçlerini belirle.
- [ ] WSC-31.3: Tarama girdilerine karşı SSRF, kaynak tüketimi, kötü amaçlı dosya ve kod yürütme sınırları kur.
- [ ] WSC-31.4: Uygulama, veri tabanı, kuyruk, indeks, çalışan görev ve bildirim sağlık ölçümlerini izle.
- [ ] WSC-31.5: Kendi pentest/bağımlılık denetimini, kurtarma tatbikatını, olay loglarını ve güvenlik bildirim kanalını işlet.

Kabul kanıtı: Merkez müşteri verisini ayırıyor; arıza ve geri yükleme senaryoları ölçülmüş.

Teknik dayanak: S13, S14.

## 32. AI ajanları ve Koschei Lang güvenliği

Kapsam: AI/ajan/Lang kullanılıyorsa.

- [ ] WSC-32.1: AI açıklamasını kanonik kanıt, doğrulanmış araç çıktısı ve sürümlü kurallara bağla.
- [ ] WSC-32.2: URL, token metadata, kaynak kod ve belgelerdeki prompt injection'ı güvenilmeyen girdi say.
- [ ] WSC-32.3: Ajanlara dar araç yetkisi, hedef sınırı, kaynak bütçesi ve süre sınırı uygula.
- [ ] WSC-32.4: Lang/derleyici/runtime çıktısını sürümle, test et ve izole ortamda doğrula.
- [ ] WSC-32.5: Canlı imza, yayın, müdahale ve yetki değişimini ayrı politika/onayla yönet; her araç çalışmasını kaydet.

Kabul kanıtı: Ajan uydurma kanıt veya yetki üretemiyor; analizden canlı işleme geçiş denetlenebiliyor.

Teknik dayanak: S15, S16.

## 33. Kalite, kıyaslama ve üretim kabulü

Kapsam: Çekirdek.

- [ ] WSC-33.1: Bilinen saldırı, normal kullanım ve zor sınır durumlarını içeren sürümlü örnek kümesi oluştur.
- [ ] WSC-33.2: Yanlış pozitif, yanlış negatif, kapsama, gecikme ve tarama maliyetini ölç.
- [ ] WSC-33.3: Mock test yanında gerçek sağlayıcı entegrasyonu ve üretime benzer uçtan uca test uygula.
- [ ] WSC-33.4: RPC kesintisi, eksik tarihçe, bozuk ayrıştırma, kayıt hatası ve durum değişimini sınayıp sonucu kontrol et.
- [ ] WSC-33.5: Yeni sürümü önceki sürümle karşılaştır; bağımsız inceleme, regresyon ve geri alma kapısı uygula.

Kabul kanıtı: Çalışıyor iddiası ölçülen senaryolarla destekli; yeşil test sayısı tek başına kabul ölçütü değil.

Teknik dayanak: S2, S10.

## 34. Ek zincir ve uzmanlık adaptörleri

Kapsam: İlan edilen destek kapsamına göre.

- [ ] WSC-34.1: Bitcoin/UTXO için script, PSBT, harcama koşulları, timelock ve işlem kesinliği kontrollerini tanımla.
- [ ] WSC-34.2: Move tabanlı ağlarda object/resource/capability, upgrade ve ortak durum güvenliğini zincire uygun değerlendir.
- [ ] WSC-34.3: Cosmos/IBC ve diğer köprülerde istemci, mesaj ve finality modeline özel doğrulama uygula.
- [ ] WSC-34.4: Validator/staking, restaking, kurumsal custody, RWA veya oyun/DePIN için ayrı tehdit modeli ekle.
- [ ] WSC-34.5: Her adaptöre desteklenen hedef/işlem matrisi ve kendi test kümesini ver; bir ağın sonucu diğerine taşınmasın.

Kabul kanıtı: Pazarlamada ilan edilen her ağ/uzmanlık gerçek adaptör ve kanıtlı testlerle eşleşiyor.

## Her kontrol için tutulacak kabul kaydı

| Alan | Kaydedilecek bilgi |
|---|---|
| Kontrol | WSC veya G kimliği ve kontrolün sürümü |
| Uygulanabilirlik | Uygulanır, uygulanamaz veya desteklenmiyor; gerekçesi |
| Hedef | Ağ, adres/URL/depo, commit veya deploy sürümü |
| Veri | Kaynak, zaman, blok/slot, incelenen dönem ve sınırlar |
| Çalıştırma | Gerçek endpoint/worker, araç sürümü ve run kimliği |
| Beklenti | Deneyde görülmesi gereken bulgu veya normal sonuç |
| Gözlenen sonuç | Yapılan kontroller, bulgu ve eksik kalan kanıt |
| Kanıt bağlantısı | İşlem, log, artefakt, yeniden üretim adımı ve hash |
| Karar | Risk, güven, öneri ve uygulanmış aksiyon ayrı alanlar |
| İnceleme | Sorumlu, inceleyen, tarih ve takip işi |

## Koschei için önerilen ürün sorumlulukları

- ARVIS: hedef araştırması, kanıt toplama, ilişki grafiği, sürümlü bulgular ve açıklanabilir inceleme dosyası.
- Sentinel: sürekli izleme, alarm, entegre işlem politikası, olay takibi ve yetkili müdahale.
- Koschei Lang / ajan yürütme katmanı: tekrarlanabilir analiz iş akışları, kontrollü araç çağrıları, yalıtılmış test ve denetlenebilir çalıştırma.
- Ortak çekirdek: kimlik, veri/kanıt şeması, kural sürümü, kanonik sonuç, saklama, sağlık ve kalite ölçümleri.

Bu dağılım bir hedef mimari önerisidir; mevcut uygulama mimarisinin doğrulandığı anlamına gelmez.

## Teknik kaynaklar

Erişim tarihi: 15 Ekim 2026. Kaynakların kapsamı farklıdır; EVM kaynağı Solana kontrolünün yerine geçmez. Sürüm numaraları uygulama sırasında kullanılan zincir ve araç sürümüyle eşleştirilmelidir.

- S1: [OWASP Smart Contract Security Verification Standard](https://scs.owasp.org/SCSVS/)
- S2: [Ethereum — Smart contract security](https://ethereum.org/developers/docs/smart-contracts/security/)
- S3: [Security Alliance — Incident Management](https://frameworks.securityalliance.org/incident-management/overview/)
- S4: [OpenZeppelin — Access Control](https://docs.openzeppelin.com/contracts/5.x/access-control)
- S5: [Solana — Token Extensions](https://solana.com/docs/tokens/extensions)
- S6: [Uniswap — Concentrated Liquidity](https://developers.uniswap.org/docs/get-started/concepts/liquidity-providers/concentrated-liquidity)
- S7: [Solana — simulateTransaction](https://solana.com/docs/rpc/http/simulatetransaction)
- S8: [EIP-712 — Typed structured data hashing and signing](https://eips.ethereum.org/EIPS/eip-712)
- S9: [EIP-7702 — Set Code for EOAs](https://eips.ethereum.org/EIPS/eip-7702)
- S10: [OWASP — Smart Contract Security Testing Guide tests](https://scs.owasp.org/SCSTG/tests/)
- S11: [Solana — Basic Security and Validation](https://solana.com/ko/developers/courses/native-onchain-development/program-security/)
- S12: [Solana — Arbitrary CPI](https://solana.com/vi/developers/courses/program-security/arbitrary-cpi)
- S13: [OWASP — Application Security Verification Standard](https://owasp.org/projects/asvs)
- S14: [OWASP — API Security Top 10](https://api-security.owasp.org/editions/2023/en/0x11-t10/)
- S15: [OWASP GenAI — Prompt Injection](https://genai.owasp.org/llmrisk/llm01-prompt-injection/)
- S16: [OWASP GenAI — Excessive Agency](https://genai.owasp.org/llmrisk/llm062025-excessive-agency/)
