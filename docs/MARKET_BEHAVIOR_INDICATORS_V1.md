# ARVIS Market Behavior Indicators v1

## Purpose

Koschei Web3 Hub / ARVIS must surface suspicious market behavior without turning a bounded observation into an unsupported accusation.

This first deterministic layer operates on transaction rows already available to Unified Radar. It is a screening system, not a market-wide forensic verdict.

## v1 indicators

ARVIS measures:

- distinct swap traders in the observed transaction set;
- buy/sell transaction count with usable timestamps;
- rapid buy-to-sell or sell-to-buy round trips by the same observed wallet within five minutes;
- one-minute windows containing at least four swap observations across at least three distinct wallets with both buy and sell directions present;
- the ratio of rapid round-trip wallets to distinct observed swap traders.

## Output states

`insufficient_evidence`
: The bounded evidence set is too small for the market-behavior screen.

`no_bounded_pattern_observed`
: The minimum bounded coverage was present, but the v1 timing thresholds were not met.

`bounded_pattern_observed`
: At least one v1 screening threshold was met. This requires further corroboration.

## Non-claims

A `bounded_pattern_observed` result does **not** prove:

- wash trading;
- market manipulation;
- common ownership or control of wallets;
- real-world identity;
- coordinated intent;
- criminal activity.

The API therefore keeps `wash_claim`, `market_manipulation_claim`, and `same_operator_claim` false for this layer.

## Evidence policy

No evidence, no claim.

The screen is deliberately bounded to recent transaction rows already collected or stored by ARVIS. Missing history, missing venues, failed RPC reads, off-chain activity and unsupported networks remain explicit coverage limits.

Future versions may strengthen the signal with market-wide venue coverage, amount/price matching, cyclic counterparty flow, liquidity/volume divergence, shared funding evidence, actor memory and persistent cross-token recurrence. Those additions must remain provenance-bearing and independently testable.
