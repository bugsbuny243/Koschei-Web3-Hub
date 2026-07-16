# Koschei Threat Anticipation Roadmap

Canonical product contract: `ACTOR_INVESTIGATION_ENGINE.md` sections 1–6.  
Current deterministic verdict ruleset: `koschei-unified-radar-rules-v1.0.0`.  
Threat pathway contract: `koschei-threat-anticipation-v1.0.0`.

## Product outcome

Koschei must answer four different questions without collapsing them into one accusation:

1. **What is verified now?** Owner concentration, authority state, observed liquidity, actor relations and parsed behavior evidence.
2. **What can the observed control surface technically do?** Market-impact capacity and technically open pathways.
3. **Which pathway is open, closed, observed, watch-only or still unknown?** Dominant-holder exit, mint inflation, freeze abuse, liquidity removal, coordinated holder exit and creator sell acceleration.
4. **Which next on-chain event would change the case?** Balance reduction, wallet fragmentation, first DEX exit, LP movement/unlock, reserve decline or linked-holder same-window exit.

The system does not predict intent, identify a real person, assign a numeric rug probability or allow an AI model to change the signed deterministic verdict.

## Repository audit

### Already strong

- Owner-resolved holder concentration and protocol/burn exclusions.
- Bounded holder funding, acquisition and flow observations.
- Shared funder, synchronized acquisition and common-exit evidence.
- Creator sell-window rules and persistent repeat-dominant-holder memory.
- Versioned, scoreless deterministic verdict with signed evidence policy.

### Partial

- Liquidity movement has market-liquidity context and program evidence, but not complete LP control attribution.
- Watch intelligence has snapshots and actor memory, but not a single persisted threat-case state machine.
- Transaction intent and MEV context exist, but reserve-level exit simulation is not yet available.

### Missing for strongest claims

- LP mint owner and beneficial-control resolution.
- Burn/locker proof, lock provider and unlock timestamp.
- Parsed add/remove-liquidity signatures with reserve deltas.
- Token and quote reserve balances for constant-product sell-impact simulation.
- Durable threat-case snapshots and alert delivery tied to evidence hashes.
- Persisted Pro/Enterprise court artifacts and an actually wired multi-provider court client.

## Phase 1 — deterministic pathway report (implemented in this change)

- Build a read-only `threat_anticipation` report from current ARVIS evidence.
- Calculate dominant-owner market-impact capacity from owner-resolved holdings and observed liquidity.
- Classify pathways as `open`, `closed`, `observed`, `watch`, `limited`, `not_observed` or `unknown`.
- Keep liquidity control `unknown` until LP burn/lock/owner evidence exists.
- Emit capability/watch scenarios and exact missing-evidence requests.
- Expose the report through shared token scans, premium Radar detail and Owner unified Radar.
- Render the report as an evidence section, not as a model-written accusation.

## Phase 2 — LP control and liquidity-drain attribution

Add a canonical LP evidence object:

- pool address and DEX program,
- LP mint,
- LP token owner/controller,
- LP supply and holder distribution,
- burned amount and burn signature,
- locker program/provider,
- locked percentage,
- unlock timestamp and unlock transaction,
- add/remove-liquidity signatures,
- token/quote reserve balances before and after each event,
- creator/dominant-holder relationship evidence.

Only transaction-backed `VERIFIED` liquidity removal may trigger a hard rule. Market-liquidity USD alone never proves that liquidity is locked, unlocked or removable.

## Phase 3 — deterministic Exit Impact Simulator

Use actual pool reserves and route-specific AMM mathematics. Do not estimate price impact from displayed market cap or Dexscreener USD liquidity alone.

For configurable supply-sale slices, produce:

- estimated quote asset received,
- price impact and slippage,
- reserve state before/after,
- percentage of holder inventory that can realistically exit,
- route and fee assumptions,
- exact evidence timestamp and pool address.

Every result must be labelled a bounded simulation, not guaranteed proceeds.

## Phase 4 — durable early warning

Persist a threat-case snapshot keyed by target, ruleset version and evidence hash. Re-open the case only when material evidence changes.

Initial alert triggers:

- dominant owner cumulative reduction reaches 1% of total supply,
- dominant owner fragments inventory across multiple new wallets,
- first parsed DEX/aggregator exit appears,
- linked holders exit in the same observation window,
- LP tokens move, unlock or burn status changes,
- parsed remove-liquidity instruction appears,
- pool reserves decline without matching ordinary trade flow,
- creator sell acceleration crosses the deterministic threshold,
- the same actor appears as creator or dominant holder in another token.

Background scanning remains opt-in and RPC-budgeted. Missing or stale data never becomes a positive safety signal.

## Phase 5 — Pro prosecution layer

The two prosecutor models receive an immutable, read-only evidence packet containing:

- signed deterministic verdict,
- threat pathway report,
- canonical evidence rows,
- exhibit data references,
- explicit missing evidence and limitations.

They may prepare an indictment and independent opinion. They cannot create evidence, change a grade or convert an `INFERRED` relation into a verified claim. The first-instance panel runs only when deterministic triggers or prosecutor disagreement require it.

## Phase 6 — Enterprise court file and exhibits

The upper panel receives the completed lower-case packet, not raw unrestricted chain history. Output includes:

- majority opinion and dissent,
- fact/inference/limitation separation,
- signed verdict references,
- deterministic SVG/Canvas exhibits generated from real node, edge, reserve, holder and timeline data,
- a downloadable court-file bundle.

Generative image models may style covers or decorative material. They must not invent evidence charts.

## Acceptance gates

1. No current auth, Neon, owner-cookie, KOSCH entitlement or route-tier behavior changes.
2. Free Safe Check still makes zero model calls.
3. No numeric rug probability or model-produced grade is introduced.
4. Protocol inventory is never treated as a risk-bearing personal holder.
5. Unknown liquidity control remains visibly unknown.
6. Every open/observed pathway points to current evidence keys or lists the exact evidence still required.
7. Missing data never produces a safe conclusion.
8. Existing signed deterministic verdict remains the sole authoritative verdict.
