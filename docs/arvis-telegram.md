# ARVIS Telegram result delivery

Koschei Telegram is a secondary delivery surface for customer ARVIS investigations. It is not a news product and it never computes, replaces or upgrades the canonical ARVIS verdict.

## Product contract

1. The customer authenticates to Koschei and explicitly pairs a private Telegram chat from `/account#arvis-telegram`.
2. ARVIS runs through the same canonical web investigation paths used by the customer workspace.
3. After a successful synchronous customer scan, or after a customer canonical investigation job is durably completed, the result is queued for that customer's paired Telegram account.
4. Telegram receives a compact result containing target, network, result/risk/grade when present, evidence state when known, recommendation when present, and a link back to the web workspace.
5. Missing or unverified evidence remains explicit and is never converted into a safe conclusion.
6. Notification failure never changes the ARVIS result or causes a completed investigation to become failed.

Background recursive/system investigations are not pushed to customers. Only customer-requested canonical jobs are mirrored. RSS/news ingestion, scheduled crypto-news digests and public news pages are not part of the ARVIS Telegram product.

## Customer controls

Authenticated API:

- `GET /api/customer/arvis/telegram` — connection state.
- `POST /api/customer/arvis/telegram/pair` — create a one-use pairing challenge after explicit consent.
- `PATCH /api/customer/arvis/telegram` — pause/resume delivery.
- `DELETE /api/customer/arvis/telegram` — disconnect the recipient.

Telegram commands:

- `/son` — resend the latest completed customer ARVIS investigation.
- `/dur` — pause result delivery.
- `/devam` — resume result delivery.
- `/sil` — disconnect Telegram from the Koschei account.

The pairing token is cryptographically random, one-use and short-lived. Only its hash is stored. Recipient identifiers and provider credentials must not be printed in logs or operator responses.

## Webhook and provider behavior

Canonical webhook:

`https://tradepigloball.co/integrations/arvis/telegram`

The runtime verifies bot identity and the Telegram webhook configuration before delivery. A one-deploy compatibility route remains at the former Crypto Brief webhook path only so Telegram can move cleanly to the canonical ARVIS URL; it is not a news endpoint.

Telegram Bot API acknowledgement proves provider acceptance, not device delivery/read status. Ambiguous in-flight sends are marked uncertain rather than blindly replayed.

## Configuration compatibility

The current production secret names retain the historical `KOSCHEI_CRYPTO_BRIEF_` prefix so credentials do not need to be copied or exposed during this migration. They now configure ARVIS Telegram result delivery only. A later persistence/config cleanup may rename them with protected in-platform secret migration.

## Legacy persistence

Migration `133_crypto_brief.sql` created the current pairing and delivery tables. Their historical names are retained temporarily as a compatibility persistence layer while the wider Neon/PostgreSQL to ClickHouse architecture is being migrated. RSS/news tables are not part of the active ARVIS Telegram runtime.

## Release invariants

- Pairing, queue deduplication, pause/resume/disconnect, webhook verification and uncertain-send fencing are covered by PostgreSQL integration tests.
- The public Crypto Brief page and its frontend assets remain retired.
- CI must stay gofmt-clean and the generated OpenAPI contract must match the registered ARVIS Telegram routes.

Do not reintroduce WhatsApp or RSS/news scheduling into this product path.
