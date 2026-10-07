# Koschei Crypto Brief v1

Customer news notifications live at `/crypto-brief`; operational source, channel and queue status lives at `/owner/crypto-brief`. This independent module does not import TradePI agent packages or ARVIS internals and never changes a security verdict.

## Delivered behavior

- Source-labelled headlines and publication dates from Cointelegraph RSS, Ethereum Foundation RSS and Solana Status Atom. No invented article summaries, signals or on-chain claims. Headlines remain in the publisher's language. Polling runs every 30 minutes; worker recovery is bounded to 15 minutes.
- Authenticated customer preferences: Bitcoin, Ethereum, Solana, global news; security, network status, DeFi, regulation, market and general topics; daily or hourly digests; timezone and quiet hours. Daily digests normally run at 09:00 in the selected timezone. Up to five matching headlines per digest, with the original publisher links. There are no price or portfolio alert adapters in v1.
- Telegram opt-in through a 10-minute, one-use, cryptographically random connection code. Only the code hash is persisted. The website explicitly asks for notification consent and binding completes from the customer's authenticated private Telegram message. An existing recipient cannot be transferred to another customer by presenting a new pairing code.
- `/gundem`, `/dur`, `/devam`, `/sil`. Pause cancels pending messages; disconnect removes the recipient and erases saved delivery text. Operator responses and logs contain no channel recipient, webhook credential or bot token.
- PostgreSQL delivery queue, atomic `SKIP LOCKED` claims, two-minute leases, fenced acknowledgements, per-subscription headline dedup, reply pacing and queue cancellation on changed preferences. Ambiguous sends and expired sending leases become `uncertain`; they are not blindly replayed because Telegram has no exactly-once key. A pause cannot retract an external request already in flight.
- Telegram Bot API acceptance is `accepted`; Telegram does not provide a delivered/read receipt for ordinary bot messages. Configured credentials and a verified webhook do not prove real customer delivery.
- Periodic bounded retention removes old headlines after 14 days and delivery/inbound replay metadata after 30 days. Expired pairings are cleaned after one day. Large backlogs may take additional cleanup cycles; no immediate deletion SLA is claimed.

## WhatsApp retirement

Crypto Brief WhatsApp delivery was retired on 2026-10-05. The customer page, operator page and registered Crypto Brief webhook surface are Telegram-only. TradePI Agent WhatsApp routes are a separate product module and are intentionally untouched. The additive v1 database migration is preserved for rollback/history compatibility; old schema fields do not make WhatsApp an active Crypto Brief channel.

## Deployment setup

Migration `133_crypto_brief.sql` is additive and applied by the normal application migration runner. Rollback is `KOSCHEI_CRYPTO_BRIEF_ENABLED=0`; preserve the tables and customer consent history.

Use a **dedicated Crypto Brief Telegram bot**. The existing `/webhooks/telegram` and `/webhooks/whatsapp` belong to TradePI agents and must not be moved.

Set configuration directly in Railway's protected variable editor. Do not send credentials through ChatGPT, put them in source files, or print them in logs.

| Variable prefix: `KOSCHEI_CRYPTO_BRIEF_` | Purpose |
|---|---|
| `ENABLED` | `1` starts public news polling and customer delivery workers. |
| `TELEGRAM_BOT_TOKEN` | Dedicated BotFather bot token, runtime secret. |
| `TELEGRAM_USERNAME` | Dedicated public bot username, without `@`. |
| `TELEGRAM_WEBHOOK_SECRET` | 32–256 characters from `A-Z`, `a-z`, `0-9`, `_`, `-`; provided as Telegram's webhook header secret. |

After setting the dedicated Telegram variables in Railway and deploying, the background runtime automatically verifies the bot identity, registers `https://tradepigloball.co/integrations/crypto-brief/telegram`, and reads the provider configuration back. It never replaces another webhook, discards pending updates, or prints credentials. Telegram deliveries remain queued until this verification succeeds. The operator runtime panel includes `crypto-brief-telegram-webhook`; a successful cycle proves the configured webhook, not receipt of a message. Verification repeats hourly; failed setup retries after 15 minutes and blocks Telegram sending until successful verification. The standalone `go run ./cmd/crypto-brief-telegram` command remains available in the trusted deployment environment as a manual check using the same validation.

For customer acceptance, log into `/crypto-brief`, choose preferences, check consent and connect Telegram. Verify an actual message on the customer's device plus `/gundem`, `/dur`, `/devam` and `/sil`. Record provider acceptance and operator queue state without copying customer identifiers. Production webhook verification is already observable; real-device delivery remains a separate acceptance step until performed.

## Validation

Unit/race tests cover bounded XML parsing, publisher URL validation, dated provenance, Atom/RSS filtering, timezone/quiet hours, Telegram webhook authentication and ambiguous provider outcomes. The native PostgreSQL 17 CI test verifies single-use hashed codes, channel/customer isolation, digest dedup, opt-out cancellation, claim exclusion, crash uncertainty and removal of recipient/text. All fixtures are explicitly synthetic; they do not prove a real external message was received.

Primary API references: [Telegram Bot API](https://core.telegram.org/bots/api) and [Telegram bot deep links](https://core.telegram.org/bots/features#deep-linking).
