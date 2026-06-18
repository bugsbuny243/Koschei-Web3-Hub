# KOSCHEİ WEB3 — Arvıs Radar Module Map

This document locks the customer-facing product direction.

## Product naming rules

```text
Site / product name: KOSCHEİ WEB3
Radar name: Arvıs
```

Customer-facing pages must not present separate technical modules as separate products. All radar systems are internal arms of Arvıs.

## Arvıs principle

Arvıs is an octopus-style Solana radar. Every radar arm has its own job, but the customer sees one unified radar surface:

```text
KOSCHEİ WEB3
  -> Arvıs Radar
      -> live network scan
      -> evidence cards
      -> evidence graph
      -> verdict/action
```

The customer should not need to understand internal module names. The customer should understand what happened, why it matters, and what action to take.

## Internal radar arms

```text
01. Live Network Scanner
    Solana WSS/RPC intake, stream heartbeat, raw events, recognized events.

02. Pump.fun Launch Intelligence
    New launch hints, early buyer timing, sniper timing, creator relation, funding cluster hints.

03. Raydium Pool Guardian
    Pool creation, pool/mint relation, LP concentration, liquidity movement.

04. Token Authority Scanner
    Mint authority, freeze authority, supply, token account state.

05. Holder Concentration Scanner
    Largest holder, top 10 holders, supply distribution, concentration risk.

06. Wallet Flow / Sybil Cluster Scanner
    Shared funding source, linked wallets, synchronized buys, cluster hints.

07. Transaction Decoder
    Signer, transfer intent, program call surface, instruction-level risk.

08. Liquidity / Rug Radar
    Liquidity drain, LP movement, authority + liquidity combined risk.

09. Project / Metadata Radar
    Project name, symbol, URL, claim surface, social/metadata signals.

10. Claim / Unsafe Instruction Shield
    Claim URL, unsafe instruction, program relation, walletless interaction risk.

11. Evidence Graph
    Wallet -> token -> pool -> holders -> risk relation map.

12. Verdict Engine
    Monitor / Watch / Manual Review / Avoid decision layer.
```

## Internal names that should not be customer-facing

These may exist in code, logs, or internal signals, but they should not be shown as product modules on the customer UI:

```text
tx_decoder
token_scanner
wallet_score
risk_scanner
sybil_graph
project_radar
walletless_claim_shield
pump_sybil_radar
raydium_pool_guardian
SBX-1
```

## Customer card contract

Arvıs cards should explain events like this:

```text
Token / Project:
Detected Source:
Creator / Wallet:
Mint Authority:
Freeze Authority:
Holder Risk:
Wallet Flow:
Liquidity:
Arvıs Verdict:
Why:
Evidence:
```

## Display language

Allowed customer-facing wording:

```text
KOSCHEİ WEB3
Arvıs Radar
Arvıs Live Network Data
Arvıs Evidence Card
Arvıs Verdict
Detected Source
Monitor
Watch
Manual Review
Avoid
No critical risk evidence found yet
Insufficient evidence
```

Avoid customer-facing wording:

```text
Trusted Token
Guaranteed Safe
Safe to Buy
Scam Token as certainty without evidence
SBX-1 as main radar name
tx_decoder as visible module
wallet_score as visible module
risk_scanner as visible module
```

## UI rule

The UI should show live data even when verdict cards are empty:

```text
Raw Events
Recognized Events
Enriched Mints
Visible Verdicts
Last Event
Last Signature
```

This proves Arvıs is live without showing low-confidence spam.

## Feed rule

Low-confidence raw stream events stay hidden from customer cards. They remain internal evidence.

A customer card can appear when:

```text
- transaction enriched mint exists
- live RPC evidence exists
- meaningful risk exists
- medium/high/critical evidence exists
```

Green/monitor never means guaranteed safety. It only means no critical risk evidence has been found in the current evidence window.
