# Koschei ARVIS Launch Day Operator Checklist

Target launch: **2026-07-01 20:00 Europe/Istanbul**

This checklist is for the wallet/operator flow only. It does not require or record seed phrases, private keys, hidden allocations, price promises or artificial volume.

## Fixed public facts

- Token name: **Koschei ARVIS**
- Symbol: **KOSCHEI**
- Launch wallet: `6ue2XG6mzY8gLvRCF3yrwe5rLB8HvfBhiWK13smahYkg`
- Treasury wallet: `DMAz2dHdiE7NKyBeUpNYyLt7B8YQEg27wD2atEwMfnPj`
- Risk disclosure: `https://tradepigloball.co/token-disclosure`
- Founder/treasury disclosure: `https://tradepigloball.co/token-vesting`
- Token gate at launch: disabled
- Automatic burn at launch: disabled

## Before the SOL withdrawal hold ends

- Do not change Railway variables except emergency fixes.
- Do not change token name or symbol.
- Do not enable token gate.
- Do not enable automatic burn.
- Do not publish any mint address.
- Do not reply to DMs that ask for wallet connection, pre-sale, private launch or seed words.
- Keep the launch wallet and treasury wallet seed phrases offline.

## When the SOL withdrawal hold ends

1. Open the exchange/app that holds SOL.
2. Choose **withdraw/send SOL on Solana network**.
3. Destination must be exactly:

   ```text
   6ue2XG6mzY8gLvRCF3yrwe5rLB8HvfBhiWK13smahYkg
   ```

4. Confirm network is **Solana**, not another chain.
5. Review the withdrawal fee and net amount.
6. Send only after the final confirmation screen matches the address above.
7. Wait until Phantom shows the received SOL in the Launch wallet.
8. Do not send SOL to the treasury wallet for launch.

## Immediately before token creation

- Open `https://tradepigloball.co/api/public/token/readiness`.
- Expected blocker before creation: only `mint: missing`.
- If identity, treasury, disclosure, vesting, launch time, burn or token gate is blocking, stop.
- Confirm the launch wallet shown in Phantom is **Koschei Launch**.
- Confirm available SOL is enough for creation, first buy and transaction fees.

## Token creation signing rules

Proceed only if the signing UI shows:

- name: **Koschei ARVIS**
- symbol: **KOSCHEI**
- wallet: `6ue2XG6mzY8gLvRCF3yrwe5rLB8HvfBhiWK13smahYkg`
- transaction type matches token creation / first buy
- no unexpected authority transfer, drain, approval or unlimited allowance

Stop immediately if:

- a website asks for seed phrase or private key;
- the wallet prompt requests an unexpected transfer;
- the displayed name or symbol differs;
- the mint cannot be copied from the official completion screen;
- the website domain is not the intended launch platform;
- the transaction simulation looks different from the expected creation/first-buy flow.

## After token creation

1. Copy the official mint address from the completed launch flow.
2. Verify the mint in at least one Solana explorer.
3. Check the address character by character before using it anywhere.
4. Set Railway variable:

   ```text
   KOSCHEI_TOKEN_MINT=<official mint address>
   ```

5. Redeploy Railway.
6. Open readiness endpoint:

   ```text
   https://tradepigloball.co/api/public/token/readiness
   ```

7. Continue only if readiness returns `launch_ready: true`.
8. Publish the mint on the Koschei transparency page before social channels.
9. Then publish X and Telegram announcements.

## First public announcement template

```text
Koschei ARVIS is live.

Name: Koschei ARVIS
Symbol: KOSCHEI
Official mint: <MINT>
Website: https://tradepigloball.co/token-disclosure
Vesting / treasury: https://tradepigloball.co/token-vesting

Koschei ARVIS is a utility and community token for Solana on-chain risk intelligence.
No guaranteed returns. No private sale. No hidden mint. Verify the mint before interacting.
```

## Anti-scam pinned reply

```text
Security notice:

The only official Koschei ARVIS mint is:
<MINT>

We will never ask for seed phrases, private keys, remote access, or private wallet approvals.
Do not trust DMs, fake airdrops, fake support accounts or copied token names.
```

## Abort conditions

Abort or delay launch if any of these happen:

- the SOL withdrawal hold remains active;
- Launch wallet receives less SOL than required;
- readiness has any blocker other than mint before token creation;
- wallet prompt differs from expected creation/first-buy flow;
- official channels show different mint addresses;
- Railway deploy fails after setting `KOSCHEI_TOKEN_MINT`;
- readiness does not return `launch_ready: true` after mint configuration;
- RPC cannot verify mint, supply or authority state;
- phishing DMs or fake mint links become active before the official post.
