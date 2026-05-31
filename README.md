# Koschei Web3 Hub

Koschei Web3 Hub; geliştiriciler, oyun varlıkları, NFT/token metadata çıktıları, launch sayfaları, risk şeffaflığı ve ekosistem büyümesi için tasarlanan AI destekli bir Web3 operating layer MVP'sidir.

## Ürün vizyonu

Koschei bir grant başvuru uygulaması değildir ve “grant başvurusu yapma” ürünü olarak konumlanmaz. Zincirlerin, altyapı sağlayıcılarının ve geliştirici topluluklarının destekleyebileceği bir ekosistem büyüme ve builder altyapı katmanı olarak tasarlanır.

### Güvenlik sınırları

- Private key veya seed phrase istenmez, saklanmaz ve işlenmez.
- Wallet custody yapılmaz; varlık veya fon tutulmaz.
- Token satışı, alım-satımı veya trading özelliği yoktur.
- Yatırım getirisi, token fiyatı veya garantili değer vaadi yoktur.
- `TOGETHER_API_KEY`, `ALCHEMY_API_KEY` ve `SOLANA_RPC_URL` yalnızca sunucu tarafında kullanılır. Frontend'e açık anahtar eklenmez.

## MVP rotaları

| Rota | Açıklama |
| --- | --- |
| `/` | Koschei Web3 Hub landing sayfası |
| `/hub` | Modül dashboard'u |
| `/builder` | No-code asset builder ve JSON export |
| `/metadata` | AI Metadata Studio |
| `/risk` | Informational Risk & Trust Scanner |
| `/chains` | Solana RPC sağlık kontrolü |
| `/ecosystem` | Ekosistem büyüme vizyonu |
| `/docs` | MVP dokümantasyonu |
| `/admin` | Basit intake/admin ekranı |
| `/quote/new`, `/quote/preview`, `/dashboard` | Çalışmaya devam eden eski TeklifPilot rotaları |

## Yerel kurulum

```bash
npm install
cp .env.example .env.local
npm run dev
```

Uygulamayı `http://localhost:3000` adresinde açın. Production kontrolü için:

```bash
npm run lint
npm run build
npm run start
```

## Ortam değişkenleri

Temel UI, AI veya chain bağlantısı olmadan çalışabilir. Aşağıdaki değişkenler ilgili özellikleri etkinleştirir:

- Genel: `APP_NAME`, `APP_ENV`, `NEXT_PUBLIC_APP_URL`
- AI: `AI_PROVIDER=together`, `AI_ENABLED=true`, `TOGETHER_API_KEY`, `TOGETHER_MODEL`
- Web3: `WEB3_PROVIDER=alchemy`, `ALCHEMY_API_KEY`, `SOLANA_NETWORK`, `NEXT_PUBLIC_SOLANA_NETWORK`, `SOLANA_RPC_URL`
- İleri aşama veritabanı: `DATABASE_URL`, `DIRECT_DATABASE_URL` (temel MVP UI için zorunlu değildir)
- Admin: `ADMIN_EMAIL`, `ADMIN_PASSWORD`

`NEXT_PUBLIC_ALCHEMY_API_KEY` veya `NEXT_PUBLIC_TOGETHER_API_KEY` eklemeyin. Sunucu tarafındaki `GET /api/web3/solana/health`, `SOLANA_RPC_URL` üzerinden `getVersion` JSON-RPC çağrısı yapar ve URL ya da API key döndürmez. `POST /api/ai/web3-generate`, Together yapılandırması eksikse veya çağrı başarısızsa deterministik fallback metni döndürür.

## Railway deploy notları

1. Repository'yi Railway'e bağlayın ve `.env.example` içindeki değerleri Railway Variables bölümüne ekleyin.
2. AI kullanımı için Together anahtarını, ChainOps için server-side Solana RPC URL'sini ayarlayın.
3. Admin ekranını kullanacaksanız `ADMIN_EMAIL` ve güçlü bir `ADMIN_PASSWORD` tanımlayın.
4. `npm run build` komutunun başarılı olduğunu doğrulayın.

## MVP sınırlamaları

- Builder asset kayıtları şimdilik yalnızca tarayıcı `localStorage` alanında saklanır.
- Admin ekranı tam auth sistemi değildir; server-side credential kontrolü sonrası tarayıcı oturumu kullanır.
- Risk sonucu bilgi amaçlı checklist'tir; hukuki, finansal, yatırım veya güvenlik tavsiyesi değildir.
- Metadata çıktıları yayınlanmadan önce insan tarafından incelenmelidir.
- Veritabanı CRUD akışı henüz aktif değildir.

## Yol haritası

1. Gelişmiş launch page builder
2. Güvenli intake ve waitlist iş akışları
3. Daha derin game asset şemaları
4. Provider-aware chain analytics
5. Opsiyonel veritabanı kalıcılığı
6. Developer education ve public-goods içerikleri
