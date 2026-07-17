# Liquidity Control Closure

Koschei collects protocol-specific pool evidence during full token investigations. This collector is informational and cannot change a grade, verdict signature, rule threshold or caller entitlement.

## Supported pool layouts

| Protocol | Pool identity | Vault reserves | Control model | Lock / burn evidence |
|---|---|---|---|---|
| Raydium CPMM | Pinned program owner and decoded pool account | Direct token-account balances | Fungible LP token | LP supply, resolved largest LP holders, burn-address share, creator-observed share, supported locker program |
| Raydium AMM v4 | Pinned program owner and fixed LiquidityStateV4 fields | Direct token-account balances | Fungible LP token | LP supply, resolved largest LP holders, burn-address share, creator-observed share, supported locker program |
| PumpSwap | Official Pool account layout and canonical pool index | Direct base/quote vault balances plus separately labelled virtual quote reserve | Fungible LP token | LP supply, resolved largest LP holders, burn-address share, creator-observed share |
| Meteora DAMM v2 | Official Pool account layout | Direct token A/B vault balances | Position NFT | Pool liquidity and permanent-locked liquidity are read from the pool account; no LP mint is invented |
| Meteora DLMM | Official bytemuck `LbPair` layout | Direct reserve X/Y token-account balances | Position account | Pool and reserve evidence is collected; position-owner enumeration remains withheld |

An unsupported program, account-owner mismatch, short account payload or mint mismatch remains `insufficient_evidence` or `source_unavailable`. A pool address alone never completes the collector.

## Recent liquidity movement window

A full scan inspects at most:

- 50 recent successful pool signatures;
- 20 parsed transactions.

A movement row requires all of the following:

1. The transaction references the decoded pool and its pinned program.
2. The transaction contains an explicit add, deposit, initialize-pool, remove, withdraw or lock instruction/log trace.
3. The decoded pool vault deltas are compatible with that trace.
4. The signature and slot are present.

Opposing token and quote reserve deltas are treated as a swap and are never reported as add/remove liquidity. Same-direction reserve changes without an explicit liquidity trace are also rejected.

Each accepted row contains:

- movement kind;
- signature and slot;
- observed block time;
- signer wallet when available;
- deterministic creator relation (`verified_pool_creator_signer`, `verified_investigated_creator_signer`, `verified_creator_lp_holder_signer`, or `not_observed`);
- source and destination accounts derived from the verified movement direction;
- pool and program;
- token and quote vault deltas;
- instruction types;
- evidence key.
- verification status (`VERIFIED`).

For fungible-LP pools the report also exposes the largest resolved LP-token
account, its owner wallet, observed supply share and classification (holder,
creator, burn address or supported locker). Raydium CPMM now decodes the pool
creator and quote mint from the pinned pool layout, so quote-vault reserve
deltas and creator-signer relations are evaluated against the actual pool
accounts rather than a market label. Burn and owner shares remain explicitly
bounded to the LP token accounts returned by Solana's largest-account RPC.

No observed row means only that no qualifying trace was found in the bounded window. It is not a claim about older activity.

## Position-based ownership boundary

Meteora DAMM v2 and DLMM do not use a fungible LP mint as the ownership source. Koschei therefore does not display LP supply, burn percentage or creator LP share for these models.

DAMM v2 permanent-lock liquidity is a pool-level field and can be reported directly. Enumerating every current DAMM v2 or DLMM position owner requires a separately bounded position index. This PR does not issue an unbounded `getProgramAccounts` request during a user scan and does not infer position ownership from pool data.

Recent add/remove transaction signers are still shown as observed transaction actors. They are not asserted to be the current owner of every position.

## Product invariants

- Capability is not intent.
- No identity attribution beyond observed on-chain wallet relations.
- No BLOCK/ALLOW or buy/sell recommendation.
- No numeric risk score or rug probability.
- Safe Check and preflight perform no new pool-history calls.
- Public, owner and API full scans use the same technical evidence path.
- Tiers affect access and capacity, not the technical result.
