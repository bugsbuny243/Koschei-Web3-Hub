# Unified Koschei work status — 2026-10-05

## Completed on integration branch

- Unified product contract now models Web3 Hub + ARVIS + Sentinel + Lang under one Professional entitlement.
- KOSC is explicitly the single official ecosystem token; separate Sentinel/Lang tokens are disabled by contract.
- Professional pricing includes Koschei Lang licensed developer tooling.
- Existing customer Telegram pairing is reused for ARVIS scan delivery.
- Every successful customer ARVIS scan can queue a customer-specific Telegram result, including evidence-pending results.
- Sentinel Web3 client added with strict `sentinel.case.v1` projection and observe-only response semantics.
- Sentinel case creation requires signed ARVIS verdict + live evidence + concrete evidence statements.
- Sentinel response case/signature mismatch is rejected.

## In progress

- Authenticated Sentinel service boundary and matching Web3 Bearer token.
- Sentinel runtime deployment configuration.
- Lang repository commercial-package contract update.
- CI/acceptance validation.

## Not yet claimed complete

- Sentinel production deployment is not claimed until service URL/token are configured and acceptance passes.
- Lang artifact distribution/download is not claimed until a licensed artifact channel exists.
- Production Telegram delivery is not claimed until bot configuration/webhook verification and a real paired-customer acceptance run pass.
