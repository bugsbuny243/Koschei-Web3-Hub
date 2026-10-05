# Crypto Brief WhatsApp retirement — 2026-10-05

Crypto Brief customer notifications are now Telegram-only.

- Removed the registered `/integrations/crypto-brief/whatsapp` product route.
- Removed WhatsApp from the Crypto Brief customer UI and preferences flow.
- Removed WhatsApp readiness/template status from the Crypto Brief owner operations UI.
- Updated customer UI tests to require Telegram-only pairing and preference writes.
- Updated `docs/crypto-brief.md` to document Telegram as the active delivery channel.
- TradePI Agent WhatsApp functionality is a separate module and was intentionally not changed.

The existing additive Crypto Brief database migration remains in place for rollback and historical compatibility. No WhatsApp provider credentials were present in Railway production at retirement time.
