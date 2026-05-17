# Koschei Bridge — Base Sepolia Web3 Çekirdeği

## Koschei Bridge nedir?
Koschei Bridge, geliştiriciler için Web3 entegrasyon köprüsüdür. İlk sürümde hedef, kullanıcıya private key göstermeden, backend üzerinden güvenli transaction akışı sağlayan bir Web3 developer tooling çekirdeği oluşturmaktır.

## Grant hedefi
Bu repo, Base ekosisteminden grant/demo değerlendirmesinde ciddi görünecek bir temel sağlar:
- Base Sepolia odaklı deploy/test ortamı
- Custodial invisible wallet üretim ve imzalama akışı
- Güvenlik katmanlı vault kontratı (operator + allowlist + pause + limit)

## Base Sepolia geliştirme ortamı
- Network: `baseSepolia`
- Chain ID: `84532`
- Hardhat deploy + verify scriptleri hazırdır.

## Custodial invisible wallet modeli
- Wallet üretimi backend scriptleriyle yapılır.
- Raw private key/mnemonic asla loglanmaz.
- Private key, AES-256-GCM ile şifrelenmiş payload olarak tutulur.
- Transaction imzalama şifreli private key çözülerek server tarafında yapılır.

## Security model
- Private key frontend'e gitmez.
- Private key/mnemonic console log yapılmaz.
- Vault tarafında recipient allowlist zorunlu.
- Withdraw işlemleri pause mekanizmasıyla durdurulabilir.
- Native ETH withdraw için günlük limit uygulanır.

## Environment variables
Aşağıdaki alanları `.env` dosyanıza ekleyin:

```env
BASE_SEPOLIA_RPC_URL=
BASE_SEPOLIA_CHAIN_ID=84532
DEPLOYER_PRIVATE_KEY=
BASESCAN_API_KEY=
KOSCHEI_WALLET_ENCRYPTION_KEY=

VAULT_ADDRESS=
OPERATOR_ADDRESS=
ALLOW_OPERATOR=true
RECIPIENT_ADDRESS=
ALLOW_RECIPIENT=true
INITIAL_NATIVE_DAILY_LIMIT_ETH=0.1

CUSTODIAL_SIGNER_ENCRYPTED_PRIVATE_KEY=

SAMPLE_TO_ADDRESS=
SAMPLE_ETH_AMOUNT=0.001
SAMPLE_NONCE=0
SAMPLE_GAS_LIMIT=21000
SAMPLE_MAX_FEE_PER_GAS=2000000000
SAMPLE_MAX_PRIORITY_FEE_PER_GAS=1000000000
SAMPLE_CHAIN_ID=84532
```

> `KOSCHEI_WALLET_ENCRYPTION_KEY` değeri 32 byte olmalıdır (örn. 64 hex karakter).

## Local setup
```bash
npm install
cp .env.example .env
```

## Compile
```bash
npm run build
```

## Test
```bash
npm run test
```

## Deploy to Base Sepolia
```bash
npm run deploy:base-sepolia
```

Çıktıda `VAULT_ADDRESS=...` satırı gelir; `.env` içine ekleyin.

## Verify on Basescan
```bash
npm run verify:base-sepolia -- <VAULT_ADDRESS> <OWNER_ADDRESS> <DAILY_LIMIT_WEI>
```

## Configure vault
```bash
npm run vault:configure:base-sepolia
```

Bu script:
- operator ekler/kaldırır
- recipient allowlist günceller
- daily native ETH limitini ayarlar

## Generate encrypted wallet
```bash
npm run wallet:generate
```

Çıktı alanları:
- `address`
- `encryptedPrivateKey`
- `chain`
- `createdAt`

## Sign transaction using encrypted wallet
```bash
npm run wallet:sign
```

Script, `CUSTODIAL_SIGNER_ENCRYPTED_PRIVATE_KEY` değerini decrypt ederek transaction imzalar.

## Mainnet readiness checklist
- Smart contract audit
- On-chain & off-chain monitoring/alerting
- Key rotation politikası
- HSM/KMS tabanlı key custody
- Incident response playbook
- Policy/risk engine

---

**Önemli not:** Bu repo şu an **Base Sepolia geliştirme çekirdeğidir**. Mainnet kullanım için audit, monitoring, key rotation, HSM/KMS ve production custody güvenliği gerekir.
