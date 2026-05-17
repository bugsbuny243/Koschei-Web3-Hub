# SECURITY — Koschei Bridge Web3 Core

## Private key policy
- Raw private key hiçbir zaman frontend'e gönderilmez.
- Raw private key hiçbir zaman plaintext dosyada veya database'de saklanmamalıdır.
- Raw private key yalnızca server process memory'de, imzalama anında kısa süreli kullanılmalıdır.

## Encryption key policy
- `KOSCHEI_WALLET_ENCRYPTION_KEY` 32 byte olmalıdır.
- Üretim ortamında environment secret manager kullanılmalıdır.
- Encryption key kaynak koda, git history'e veya client bundle'a girmemelidir.

## No frontend private key rule
- Client tarafına private key, mnemonic, seed phrase ve decrypt anahtarı taşınmaz.
- Wallet operasyonlarının tamamı backend/service katmanında çalışmalıdır.

## No console logging secrets rule
- Private key, mnemonic, seed phrase, encryption key ve decrypt edilmiş secret loglanmamalıdır.
- Debug logları secrets içermeyecek şekilde sanitize edilmelidir.

## Custodial wallet riskleri
- Custodial modelde anahtar riski platformdadır.
- Compromise durumunda kullanıcı fonları tehlikeye girebilir.
- Bu nedenle erişim kontrolü, operasyonel süreçler ve monitoring zorunludur.

## Mainnet öncesi yapılacaklar
- Bağımsız smart contract audit
- Threat modeling ve attack simulation
- Operational runbook ve incident tabletop
- Production secret management hardening

## Audit ihtiyacı
- Vault contract için en az bir bağımsız audit yapılmalıdır.
- Kritik bulgular giderilmeden mainnet release yapılmamalıdır.

## Key rotation planı
- Encryption key ve operator key periyodik olarak rotate edilmelidir.
- Rotation sırasında encrypted key payloadları güvenli şekilde yeniden şifrelenmelidir.
- Eski anahtarlar kontrollü ve izlenebilir şekilde devreden çıkarılmalıdır.

## Incident response notları
- Şüpheli key leak durumunda anında pause uygulanmalıdır.
- Operator yetkileri revoke edilmeli ve allowlist gözden geçirilmelidir.
- Post-mortem + kullanıcı/partner bilgilendirme planı işletilmelidir.
