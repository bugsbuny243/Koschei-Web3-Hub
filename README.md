# TeklifPilot

TeklifPilot, Türk KOBİ'lerinin WhatsApp ve benzeri kanallardan gelen ürün taleplerini profesyonel İngilizce ihracat tekliflerine dönüştürmesine yardımcı olan yerel çalışan MVP web uygulamasıdır.

## Özellikler

- Türkçe ürün, firma ve alıcı bilgileriyle teklif oluşturma
- Together AI etkinleştirildiğinde yapay zekâ destekli, aksi durumda deterministik şablonlarla profesyonel İngilizce teklif metni üretme
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

## Together AI Yapılandırması

Together AI ile teklif metni üretimini etkinleştirmek için Railway ortam değişkenlerini aşağıdaki gibi ayarlayın:

```bash
AI_ENABLED=true
AI_PROVIDER=together
TOGETHER_API_KEY=...
TOGETHER_MODEL=Qwen/Qwen3-235B-A22B-Instruct-2507-tput
```

- `TOGETHER_API_KEY` yalnızca sunucu tarafında okunur; istemciye açık bir `NEXT_PUBLIC_` anahtarı kullanılmaz.
- `AI_ENABLED=true`, `AI_PROVIDER=together` ve `TOGETHER_API_KEY` birlikte mevcut değilse deterministik yerel şablon kullanılır. `TOGETHER_MODEL` belirtilmezse varsayılan model kullanılır.
- Together isteği başarısız olursa teklif oluşturma işlemi kesilmez; deterministik yerel şablona geri dönülür.
- Form, sunucu tarafındaki `POST /api/ai/generate-quote` route’una gider. API anahtarı tarayıcıya gönderilmez.

## Önemli MVP Notları

- Bu sürümde veritabanı veya üyelik sistemi yoktur. Together AI entegrasyonu isteğe bağlıdır.
- Oluşturulan teklifler yalnızca kullanılan tarayıcının `localStorage` alanında saklanır. Tarayıcı verileri temizlenirse kayıtlar silinir.
- PDF dosyası sunucuda oluşturulmaz. Teklif önizleme ekranında **Yazdır / PDF Kaydet** düğmesine tıklayın ve tarayıcının yazdırma penceresinden **PDF olarak kaydet** seçeneğini kullanın.
- İngilizce teklif metni, Together AI etkin değilse veya Together isteği başarısız olursa deterministik şablonlarla hazırlanır.
- HS/GTIP bilgisi tahmini yardımcı alandır; sevkiyat öncesinde yetkili gümrük müşaviri veya ilgili makam tarafından doğrulanmalıdır.

## Yol Haritası

1. WhatsApp üzerinden talep alma
2. Müşteri hesapları
3. Teklif geçmişi
4. Gerçek PDF üreticisi
5. Ödeme sayfası
6. Yönetici paneli
