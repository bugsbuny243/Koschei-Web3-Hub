# TeklifPilot

TeklifPilot, Türk KOBİ'lerinin WhatsApp ve benzeri kanallardan gelen ürün taleplerini profesyonel İngilizce ihracat tekliflerine dönüştürmesine yardımcı olan yerel çalışan MVP web uygulamasıdır.

## Özellikler

- Türkçe ürün, firma ve alıcı bilgileriyle teklif oluşturma
- Deterministik şablonlarla profesyonel İngilizce teklif metni üretme
- WhatsApp veya e-posta için hazır takip mesajı oluşturma
- Temiz, A4 uyumlu teklif önizleme sayfası
- Tarayıcı üzerinden yazdırma veya PDF olarak kaydetme
- Son teklifleri tarayıcının yerel depolamasında saklama

## Yerel Kurulum

Node.js ve npm kurulu bir ortamda proje klasöründe aşağıdaki komutları çalıştırın:

```bash
npm install
npm run dev
```

Ardından tarayıcınızda [http://localhost:3000](http://localhost:3000) adresini açın.

Production build almak için:

```bash
npm run build
npm run start
```

## Önemli MVP Notları

- Bu sürümde veritabanı, üyelik sistemi veya harici servis entegrasyonu yoktur.
- Oluşturulan teklifler yalnızca kullanılan tarayıcının `localStorage` alanında saklanır. Tarayıcı verileri temizlenirse kayıtlar silinir.
- PDF dosyası sunucuda oluşturulmaz. Teklif önizleme ekranında **Yazdır / PDF Kaydet** düğmesine tıklayın ve tarayıcının yazdırma penceresinden **PDF olarak kaydet** seçeneğini kullanın.
- İngilizce teklif metni harici yapay zekâ servisi olmadan deterministik şablonlarla hazırlanır.
- HS/GTIP bilgisi tahmini yardımcı alandır; sevkiyat öncesinde yetkili gümrük müşaviri veya ilgili makam tarafından doğrulanmalıdır.

## Yol Haritası

1. WhatsApp üzerinden talep alma
2. API anahtarıyla yapay zekâ destekli metin üretimi
3. Müşteri hesapları
4. Teklif geçmişi
5. Gerçek PDF üreticisi
6. Ödeme sayfası
7. Yönetici paneli
