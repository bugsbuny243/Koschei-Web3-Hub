# Koschei Web3 Hub

Koschei Web3 Hub; builder'lar, oyunlar, varlıklar, metadata, launch sayfaları, risk şeffaflığı ve ekosistem büyümesi için AI destekli bir Web3 operasyon katmanıdır.

> Koschei bir grant başvuru uygulaması değildir. Token trading, custody veya private-key deployment ürünü değildir. Yatırım getirisi ya da token fiyatı vaadinde bulunmaz.

## Vizyon

Koschei, fikirden geliştiriciye hazır çıktıya giden yolu kısaltır: oyun varlığı ve NFT metadata taslakları oluşturur, proje metinlerini güvenli kurallarla üretir, kamuya açık güven sinyallerini risk checklist'i ile görünür kılar ve desteklenen testnet RPC bağlantılarını sunucu tarafında kontrol eder. Ekosistem büyümesi, builder onboarding ve daha standart proje çıktıları odağındadır.

Koschei; chain'ler, altyapı sağlayıcıları ve geliştirici toplulukları tarafından desteklenebilecek bir ekosistem büyüme katmanı olarak tasarlanmıştır.

## Güvenlik İlkeleri

- Wallet bağlantısı sunulmaz.
- Wallet private key, seed phrase veya deployer key istenmez ve işlenmez.
- Fon veya token custody yapılmaz.
- Token trading özelliği yoktur.
- Gerçek token veya contract deploy edilmez.
- Yatırım getirisi ya da token fiyatı sözü verilmez.
- `TOGETHER_API_KEY` ve `ALCHEMY_API_KEY` yalnızca sunucuda okunur. Frontend'e açık `NEXT_PUBLIC_` API anahtarı oluşturulmaz.
- MVP asset çıktıları yalnızca tarayıcının `localStorage` alanında saklanır.

## MVP Route'ları

| Route | Açıklama |
| --- | --- |
| `/` | Koschei Web3 Hub landing page |
| `/hub` | Modül dashboard'u |
| `/builder` | No-code Web3 metadata ve asset concept builder |
| `/metadata` | AI Metadata Studio |
| `/risk` | Risk & Trust Scanner checklist MVP |
| `/chains` | ChainOps testnet RPC health dashboard |
| `/ecosystem` | Ekosistem büyüme vizyonu |
| `/docs` | Geliştirici dokümantasyonu |
| `/login`, `/signup` | Standart kullanıcı giriş ve kayıt route'ları |
| `/dashboard` | Giriş gerektiren kullanıcı paket ve çıktı hakkı paneli |
| `/admin` | Public navigasyonda gösterilmeyen admin-only operasyon paneli |
| `/quote/new`, `/quote/preview` | Mevcut TeklifPilot route'ları |

## Yerel Çalıştırma

```bash
npm ci
cp .env.example .env.local
npm run dev
```

`package-lock.json` commit edilmiştir. Railway ve yerel kurulumlarda kilitli bağımlılık sürümleriyle tekrarlanabilir build almak için `npm ci` kullanın.

Tarayıcıda [http://localhost:3000](http://localhost:3000) adresini açın. Production kontrolü için:

```bash
npm run check:auth-architecture
npm run lint
npm run build
npm run start
```

## Ortam Değişkenleri

Tüm desteklenen değişkenler `.env.example` dosyasındadır. İlk MVP için önerilen temel ayarlar:

```bash
APP_NAME=Koschei Web3 Hub
APP_ENV=development
NEXT_PUBLIC_APP_URL=http://localhost:3000
ADMIN_EMAIL=...
ADMIN_PASSWORD=...
AUTH_API_URL=http://localhost:8080
USER_SESSION_SECRET=long-random-secret
AI_PROVIDER=together
AI_ENABLED=false
TOGETHER_API_KEY=...
TOGETHER_MODEL=...
WEB3_PROVIDER=alchemy
ALCHEMY_API_KEY=...
SOLANA_RPC_URL=...
```

- AI opsiyoneldir. `AI_ENABLED=true`, `AI_PROVIDER=together` ve `TOGETHER_API_KEY` birlikte yoksa veya Together isteği başarısız olursa deterministik fallback metni döner.
- Chain health için birincil yapılandırma `ALCHEMY_API_KEY` değeridir. Solana için `SOLANA_RPC_URL`, EVM chain'ler için opsiyonel `*_RPC_URL` override'ları kullanılabilir; explicit RPC URL tanımlanırsa ilgili chain için önceliklidir. API key ve RPC URL değerleri yalnızca sunucuda kalır.
- `DATABASE_URL` ve `DIRECT_DATABASE_URL` Railway'deki mevcut Neon bağlantı ENV'leri olarak korunur. `DATABASE_URL`, Go `auth-api` profil upsert işlemleri ve mevcut server-side web veri route'ları için kullanılır; `NEXT_PUBLIC_` önekiyle yayınlanmaz.
- Standart kullanıcı signup/login akışında Next.js yalnızca Go `auth-api` servisine proxy olur. Go servisi Neon Auth `sign-up/email` ve `sign-in/email` endpoint'lerini çağırır, JWKS ile RS256 veya EdDSA JWT doğrulaması yapar ve yalnızca JWT içindeki `sub` ile `email` profil bilgilerini `app_user_profiles` içine upsert eder. Üye parolası, `password_hash` veya `member_accounts` tablosu kullanılmaz. Eski Next.js `src/lib/server/neon-auth.ts` helper’ı kaldırılmıştır; member credential doğrulaması yalnızca Go servisinde yapılır. Dashboard oturumu güvenli, httpOnly `koschei_member_session` cookie kullanır. `/admin` erişimi tamamen ayrıdır ve yalnızca server-side `ADMIN_EMAIL` ile `ADMIN_PASSWORD` doğrulamasını kullanır.
- Shopier ödeme doğrulaması şimdilik admin tarafından manuel yapılır. Frontend siparişleri yalnızca `pending` oluşturur; public navigasyonda gösterilmeyen admin-only `/admin` alanında ödeme doğrulanmadan entitlement aktif olmaz.

## AI Akışı

`POST /api/ai/web3-generate`, `metadata`, `description`, `pitch`, `lore` ve `launch` modlarını kabul eder. Together API isteği sunucudan yapılır. Sistem prompt'u fiyat/getiri vaadi, uydurma audit veya partnership, resmi chain/Alchemy desteği iddiası ve pump/scam dilini yasaklar. Sağlayıcı kapalıysa veya hata verirse uygulama kırılmaz; deterministik fallback kullanılır.

TeklifPilot için mevcut `POST /api/ai/generate-quote` route'u korunmuştur.

## ChainOps Akışı

`GET /api/web3/health?chain=solana|base|arbitrum|polygon|optimism|ethereum` desteklenen testnet'e JSON-RPC sağlık isteği gönderir. RPC URL veya API key response içine eklenmez. Dashboard yalnızca güvenli health sonucu, chain, network, provider ve hata veya sonucu gösterir.

## Railway Deploy Notları

Railway üzerinde iki ayrı servis kurun:

| Servis | Build / start | Gerekli değişkenler |
| --- | --- | --- |
| `web` | `npm ci && npm run build` / `npm run start` | `AUTH_API_URL`, `USER_SESSION_SECRET` ve mevcut admin / uygulama değişkenleri |
| `auth-api` | `cd services/auth-api && go build -o auth-api .` / `cd services/auth-api && ./auth-api` | `DATABASE_URL`, `NEON_AUTH_BASE_URL`, `NEON_AUTH_ISSUER`, `NEON_AUTH_JWKS_URL`, `USER_SESSION_SECRET`, `CORS_ALLOWED_ORIGIN=https://tradepigloball.co`, `APP_ENV=production` |

`AUTH_API_URL`, Railway içindeki Go servisinin internal URL değeridir. İki servis aynı `USER_SESSION_SECRET` değerini kullanmalıdır. Secret değerleri `NEXT_PUBLIC_` önekiyle yayınlamayın. Normal kullanıcı alanları yalnızca `/signup`, `/login` ve `/dashboard`; admin alanı yalnızca ayrı `/admin` route'udur. Owner Console, dashboard admin bağlantısı, Google Cloud, wallet, token trading veya custody akışı eklenmez.

## Yol haritası

1. Daha kapsamlı game asset şemaları ve doğrulama
2. Launch page çıktılarının genişletilmesi
3. Shopier webhook desteği ve daha kapsamlı production-grade auth
4. Ecosystem lead ve project onboarding workflow'ları
5. Developer integration örnekleri ve standartlaştırılmış export paketleri
6. Daha kapsamlı risk transparency kuralları

## Web3 Hub Database Migration

`sql/2026_05_31_koschei_web3_hub_schema.sql` migration'ını uygulayın. Migration mevcut tabloları veya legacy kolonları silmez. Web3 Hub paketleri mevcut `plans` tablosuna seed edilir. `entitlements`, satın alınan çıktı haklarını tutar; `web3_outputs`, üretilen metadata, risk ve launch çıktılarını saklamak içindir. Shopier ödemeleri ilk aşamada manuel doğrulanır. `DATABASE_URL` yalnızca sunucu tarafında tutulmalıdır; frontend'e açık bir `NEXT_PUBLIC_DATABASE_URL` oluşturulmaz.
