# Koschei Crypto Brief v1

Customer news notifications live at `/crypto-brief`; operational source, channel and queue status lives at `/owner/crypto-brief`. This independent module does not import TradePI agent packages or ARVIS internals and never changes a security verdict.

## Delivered behavior

- Source-labelled headlines and publication dates from Cointelegraph RSS, Ethereum Foundation RSS and Solana Status Atom. No invented article summaries, signals or on-chain claims. Headlines remain in the publisher's language. Polling runs every 30 minutes; worker recovery is bounded to 15 minutes.
- Authenticated customer preferences: Bitcoin, Ethereum, Solana, global news; security, network status, DeFi, regulation, market and general topics; daily or hourly digests; timezone and quiet hours. Daily digests normally run at 09:00 in the selected timezone. Up to five matching headlines per digest, with the original publisher links. There are no price or portfolio alert adapters in v1.
- Telegram and WhatsApp opt-in through a 10-minute, one-use, cryptographically random connection code. Only the code hash is persisted. The website explicitly asks for notification consent and binding completes from the customer's authenticated private channel message. An existing recipient cannot be transferred to another customer by presenting a new pairing code.
- `/gundem`, `/dur`, `/devam`, `/sil` (WhatsApp also accepts words without slash). Pause cancels pending messages; disconnect removes the recipient and erases saved delivery text. Operator responses and logs contain no channel recipient, webhook credential or bot token.
- PostgreSQL delivery queue, atomic `SKIP LOCKED` claims, two-minute leases, fenced acknowledgements, per-subscription headline dedup, reply pacing and queue cancellation on changed preferences. Ambiguous sends and expired sending leases become `uncertain`; they are not blindly replayed because the provider APIs have no exactly-once key. A pause cannot retract an external request already in flight.
- API acceptance is `accepted`. Authenticated WhatsApp delivery/read callbacks may promote it to `delivered`; Telegram does not provide equivalent delivery receipts. Configured credentials do not prove real customer delivery.
- Outside the verified 24-hour WhatsApp inbound window, an approved template is required. Missing configuration leaves delivery pending with `whatsapp_template_required`. Template requests use one body text parameter, bounded to 900 characters. Neutral public news does not offer trading, token sales or investment promotions.
- Periodic bounded retention removes old headlines after 14 days and delivery/inbound replay metadata after 30 days. Expired pairings are cleaned after one day. Large backlogs may take additional cleanup cycles; no immediate deletion SLA is claimed.

## Deployment setup

Migration `133_crypto_brief.sql` is additive and applied by the normal application migration runner. Rollback is `KOSCHEI_CRYPTO_BRIEF_ENABLED=0`; preserve the tables and customer consent history.

Use a **dedicated Crypto Brief Telegram bot and WhatsApp business phone**. The existing `/webhooks/telegram` and `/webhooks/whatsapp` belong to TradePI agents. A provider webhook must not be moved away from those routes.

Set configuration directly in Railway's protected variable editor. Do not send credentials through ChatGPT, put them in source files, or print them in logs.

| Variable prefix: `KOSCHEI_CRYPTO_BRIEF_` | Purpose |
|---|---|
| `ENABLED` | `1` starts public news polling and customer delivery workers. |
| `TELEGRAM_BOT_TOKEN` | Dedicated BotFather bot token, runtime secret. |
| `TELEGRAM_USERNAME` | Dedicated public bot username, without `@`. |
| `TELEGRAM_WEBHOOK_SECRET` | 32–256 characters from `A-Z`, `a-z`, `0-9`, `_`, `-`; provided as Telegram's webhook header secret. |
| `WHATSAPP_ACCESS_TOKEN` | Dedicated Meta Cloud API runtime secret. |
| `WHATSAPP_PHONE_NUMBER_ID` | Business phone number ID. |
| `WHATSAPP_NUMBER` | Public business number, international digits, no `+`. |
| `WHATSAPP_APP_SECRET` | Meta app runtime secret for HMAC verification. |
| `WHATSAPP_VERIFY_TOKEN` | At least 32 characters for Meta webhook verification. |
| `WHATSAPP_GRAPH_VERSION` | Supported Meta version for the configured app, e.g. `v23.0`; select the app's current supported version. |
| `WHATSAPP_TEMPLATE_NAME` | Meta-approved news template with exactly one body text parameter. |
| `WHATSAPP_TEMPLATE_LANGUAGE` | Approved language code; default `tr`. |

After setting the dedicated Telegram variables in Railway and deploying, the background runtime automatically verifies the bot identity, registers `https://tradepigloball.co/integrations/crypto-brief/telegram`, and reads the provider configuration back. It never replaces another webhook, discards pending updates, or prints credentials. Telegram deliveries remain queued until this verification succeeds. The operator runtime panel includes `crypto-brief-telegram-webhook`; a successful cycle proves the configured webhook, not receipt of a message. Verification repeats hourly; failed setup retries after 15 minutes and blocks Telegram sending until successful verification. The standalone `go run ./cmd/crypto-brief-telegram` command remains available in the trusted deployment environment as a manual check using the same validation.

Register Meta callback `https://tradepigloball.co/integrations/crypto-brief/whatsapp`, using the verification secret in the protected Meta console, and subscribe to messages/statuses. Meta template text may use `Kripto gündeminiz: {{1}}. Bildirimleri durdurmak için DUR yazın.`; approval must be verified in Meta, not inferred from a configured name.

For customer acceptance, log into `/crypto-brief`, choose preferences, check consent and connect a channel. Verify an actual message on the customer's device, `/gundem`, `DUR`, and `SIL`. Record the provider acceptance/receipt and operator queue state without copying customer identifiers. No production channel delivery was demonstrated during implementation because the dedicated provider credentials were absent.

## Validation

Unit/race tests cover bounded XML parsing, publisher URL validation, dated provenance, Atom/RSS filtering, timezone/quiet hours, webhook authentication, template-window enforcement and ambiguous provider outcomes. The native PostgreSQL 17 CI test verifies single-use hashed codes, channel/customer isolation, digest dedup, opt-out cancellation, claim exclusion, crash uncertainty and removal of recipient/text. All fixtures are explicitly synthetic; they do not prove a real external message was received.

Primary API references: [Telegram Bot API](https://core.telegram.org/bots/api), [Telegram bot deep links](https://core.telegram.org/bots/features#deep-linking), [Meta template payload](https://whatsapp.github.io/WhatsApp-Nodejs-SDK/api-reference/messages/template/), [WhatsApp Business Messaging Policy](https://business.whatsapp.com/policy) (checked 2026-10-04).
