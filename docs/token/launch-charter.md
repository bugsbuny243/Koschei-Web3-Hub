# Koschei Ecosystem Token Launch Charter

Status: infrastructure preparation only. The token is not live until its mint address is verified from Solana by the public status endpoint.

## Product purpose

The token may become an optional utility layer for ARVIS usage credits, partner incentives, verified contributor rewards and community product signalling. Essential security warnings must not depend entirely on token ownership.

## Safeguards

- No guaranteed return, price target or passive-income claim.
- Official mint, supply, authorities, treasury wallets, allocation, vesting and liquidity evidence must be public.
- No undisclosed supply changes. The preferred final state is fixed supply after required distribution is complete.
- Team, treasury and contributor allocations require independently verifiable vesting.
- Treasury custody must move to multisig before material value is held.
- Token ownership does not represent company equity, revenue share or a claim on company assets.
- Missing evidence is displayed as unavailable, never converted into a positive claim.

## Launch gates

- [ ] Name, symbol, decimals and maximum supply approved.
- [ ] Legal and public risk disclosures reviewed.
- [ ] Devnet mint and complete lifecycle tested.
- [ ] Authority policy approved and published.
- [ ] Treasury multisig addresses published.
- [ ] Allocation and vesting schedule published.
- [ ] Durable metadata and logo storage configured.
- [ ] Mainnet mint verified by `/api/public/token/status`.
- [ ] Holder concentration monitoring enabled.
- [ ] Liquidity evidence published.
- [ ] Compromised-key and incident procedures tested.

## Technical baseline

The system supports both SPL Token and Token-2022 verification. The final token program and any extensions will be selected only after wallet, DEX, custody and analytics compatibility checks. Optional extensions are not enabled without a written product reason.

## Public verification

`GET /api/public/token/status` is the public source of truth. Before launch it returns `phase: planning`. After configuration, it reads the mint program, supply, authorities and largest token accounts directly through Solana RPC.

Production configuration after final decisions:

- `KOSCHEI_TOKEN_MINT`
- `KOSCHEI_TOKEN_NETWORK`
- `KOSCHEI_TOKEN_NAME`
- `KOSCHEI_TOKEN_SYMBOL`
- `KOSCHEI_TOKEN_TREASURY`
- `KOSCHEI_TOKEN_DISCLOSURE_URL`
- `KOSCHEI_TOKEN_VESTING_URL`
- `KOSCHEI_TOKEN_LIQUIDITY_LOCK_URL`

Private keys must never be stored in GitHub, Neon or public environment variables.
