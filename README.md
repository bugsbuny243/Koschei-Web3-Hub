# Koschei Web3 Hub

No-custody Pro Web3 intelligence for users who need to understand transactions, wallets, tokens, portfolios, and project risk before acting.

## Current product strategy

Koschei now exposes **only six production modules**. All removed modules are intentionally unavailable from the user-facing app and API router.

1. **TX Decoder** — explains a Solana transaction signature before users sign or trust the activity.
2. **Token Scanner / Rug Checker** — checks token mint authority, freeze authority, holder concentration, and rug-risk signals.
3. **Wallet Score / Reputation** — scores public wallet activity and risk history.
4. **Risk Scanner** — general wallet + token + contract + project risk checklist.
5. **Portfolio Tracker** — tracks multiple public wallets and token/SOL balances.
6. **Project Radar** — reviews public project, metadata, social, token, and wallet signals.

## Pro-only policy

There is no demo or fabricated-data mode for active modules. Every module call requires:

- a valid authenticated user session,
- a live backend API route,
- sufficient credits for that specific module,
- public/read-only inputs only.

Koschei never asks for private keys, seed phrases, or custody of funds.

## Admin panel policy

The owner admin panel is intentionally limited to:

- member counts,
- purchased packages / payment requests,
- member credits,
- Koschei Chat.

User-facing module management screens are not shown in admin.

## Tech stack

- Go backend
- Static frontend
- Neon Postgres
- Neon Auth
- Alchemy / Solana RPC
- Together AI where enabled for admin chat and supported helpers

## Required environment categories

See `.env.example` for the full variable list. Neon Auth variables must stay in place and should not be replaced by a different auth system.

## Safety disclaimer

Koschei is informational software, not financial, legal, investment, or security advice. Always verify addresses, contracts, token authorities, and transaction details independently.
