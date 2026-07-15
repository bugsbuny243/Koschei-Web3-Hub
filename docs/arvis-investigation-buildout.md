# ARVIS Investigation Buildout

Applicable canonical sections: `ACTOR_INVESTIGATION_ENGINE.md` sections 1, 2, 3, 4, 5 and 6.  
Actor ruleset version: `koschei-actor-defense-rules-v1.0.0`.  
Unified Radar ruleset version: `koschei-unified-radar-rules-v1.0.0`.

This buildout starts from the current repository audit without deleting legacy production paths and without touching auth, Neon Auth, sessions, owner cookies, KOSCH entitlement or verified-wallet implementation.

## Evidence contract

- ARVIS remains an evidence-first actor investigation engine, not a numeric risk-card generator.
- Every new claim must answer at least one canonical ten-question filter item.
- Serious claims require evidence rows with signature, slot, timestamp, source, destination, amount, program and verification status.
- `INFERRED` evidence remains watch-only.
- `UNVERIFIED` evidence cannot affect a grade and cannot appear as a verified claim.
- The 14 legacy ARVIS arms remain intact; replacement work must prove behavioral parity and rollback safety before any deletion.

## Current investigation capability map

| Capability | Current state | Primary repository path | Next evidence need |
|---|---|---|---|
| Solana token intelligence | Strong evidence arm | Token authority, holder concentration and program relation arms | Continue attaching parsed Solana RPC observations only. |
| Holder / funding / sybil | Strong evidence arm | Holder concentration, funding cluster, Pump Sybil and sniper timing arms | Keep timing and funding signals evidence-only unless direct ownership evidence exists. |
| Creator / repeat actor memory | Partial evidence path | Creator Link Analysis and Repeat Actor Scan | Attach persistent actor-index rows without broad recipient wallet-history scans. |
| Launch / sniper intelligence | Strong evidence arm | Launch Distribution and Sniper Timing Detector | Keep launch distribution mint-specific and ATA-based. |
| Liquidity drain attribution | Partial evidence path | Liquidity Movement and Raydium Pool Guardian | Connect pool reserve deltas, LP authority and creator/dominant-holder actor relations. |
| Transaction intent | Strong evidence arm | Program Relation Scan, Claim Surface Risk and Walletless Claim Shield | Extend parsed instruction, signer, writable-account and balance-delta intent evidence with route-specific claim, swap and approval semantics. |
| MEV / sandwich | Partial evidence path | MEV Shield | Attach route, slippage and pool-state before/after observations before any sandwich claim. |
| Market manipulation | Planned evidence path | Funding cluster, liquidity and holder behavior rules | Map wash/self-flow, coordinated exits and volume/liquidity gaps into deterministic behavior rules. |
| Watch intelligence | Partial evidence path | Intelligence Graph, Repeat Actor Scan and watchlist observations | Connect watch observations to durable actor memory without making inferred evidence grade-affecting. |
| Cross-chain intelligence | Schema-only | Cross-chain and bridge schema traces | Add verified bridge/chain evidence ingestion before surfacing cross-chain criminal-pattern claims. |
| Cross-chain criminal patterns | Not a verified claim | No production evidence arm yet | Define bridge, mixer, peel-chain and stablecoin-conversion evidence-row schemas first. |

## Immediate implementation order

1. Preserve and expose the investigation capability map in ARVIS metadata so owner/research surfaces can show what is verified, partial, planned or unavailable without fabricating production claims.
2. Extend transaction-intent evidence with route-specific claim, swap and approval semantics while preserving the existing 14-arm contract.
3. Connect creator/repeat actor memory to durable actor-index rows.
4. Extend liquidity attribution with signed add/remove, reserve delta, LP authority and actor-link evidence.
5. Only after the above, add cross-chain ingestion with strict evidence rows for bridge, mixer, peel-chain, stablecoin conversion and CEX/OTC movement claims.
