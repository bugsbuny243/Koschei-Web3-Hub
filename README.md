# Koscei Bridge — Base Sepolia Web3 Geliştirme Ortamı

Bu repo, Base ekosistemi için köprü projesinin:
- akıllı kontrat geliştirme,
- Base Sepolia deploy ve doğrulama,
- custodial wallet (görünmez cüzdan) backend akışları

için hızlı başlangıç ortamını içerir.

## 1) Kurulum

```bash
npm install
cp .env.example .env
```

`.env` içinde en az aşağıdaki alanları doldurun:
- `BASE_SEPOLIA_RPC_URL`
- `DEPLOYER_PRIVATE_KEY`
- `CUSTODIAL_SIGNER_PRIVATE_KEY`

## 2) Derleme ve test

```bash
npm run build
npm run test
```

## 3) Base Sepolia deploy

```bash
npm run deploy:base-sepolia
```

Kontrat: `contracts/CustodialVault.sol`

### CustodialVault yetkileri
- `owner`: operatör ekler/çıkarır.
- `operator`: native ve ERC20 çekim işlemlerini yürütür.

Bu yapı, backend servisinizin operasyonel anahtarını (`operator`) owner anahtarından ayırarak risk azaltır.

## 4) Custodial wallet scriptleri

Yeni cüzdan üretmek için:

```bash
npm run wallet:generate
```

Offline tx imzalama örneği:

```bash
npm run wallet:sign
```

## 5) Dosya yapısı

- `hardhat.config.ts`: Base Sepolia network ve compiler ayarları
- `contracts/CustodialVault.sol`: custody operasyon kontratı
- `scripts/deploy.ts`: deploy scripti
- `src/wallet/`: custodial wallet yardımcı scriptleri
- `test/CustodialVault.ts`: temel yetki/withdraw testi

## Sonraki adımlar

- Multisig owner (Safe) ile `owner` rolünü taşıma
- Operator için HSM/KMS entegrasyonu
- Withdraw için hız limiti / allowlist / günlük limit gibi risk kontrolleri
- Event indexing ve muhasebe reconciler servisi
