# Koschei Web3 Hub — global crypto intelligence gap analysis

Date: 2026-10-05

This audit translates the standalone Web3 product mission into evidence-backed implementation priorities. It was corrected after a direct code/migration audit so existing capabilities are not rebuilt or described as missing. It does not claim a branch-only feature is production-ready before merge, migration and release gates pass.

## Product boundary

Koschei Web3 Hub is an independent commercial crypto security/intelligence product. ARVIS is its investigation, evidence-correlation, scanning and decision engine. Koschei Sentinel and Koschei Lang are separate products and are not Web3 entitlements.

## Confirmed existing foundation

The repository already contains substantial production-oriented foundations for:

- Solana token intelligence;
- token authority and holder concentration;
- holder/funding/sybil relationships;
- launch distribution and sniper timing;
- transaction intent and program-relation analysis;
- persistent funding-cluster memory;
- actor operational memory across verified relations, shared funders and repeated token contexts;
- campaign-genome memory with explicit anti-attribution boundaries;
- durable Global Campaign revisions and evidence-anchor lookup;
- durable trade-event memory through `token_trade_events`;
- deterministic market/holder behavior rules including volume/liquidity gaps, holder/liquidity pressure, creator-sell acceleration and dominant-holder exit;
- evidence graph, incident memory and deterministic verdict concepts.

These systems should be extended rather than duplicated.

## Corrected status by intelligence lane

### A. Multi-wallet / repeat-operator intelligence

Current state: **substantial foundation already implemented; customer-facing coordination projection added in this branch.**

Existing code already supports persistent funding clusters and actor operational memory. Direct verified relations remain distinct from shared-funder or operational-overlap inference. Campaign-genome matching explicitly does not prove common identity, control or wrongdoing.

Branch expansion:

- `actor_coordination_intelligence` composes the existing retained actor/funder memory instead of creating a second graph;
- repeated creator/deployer activity, repeated dominant-holder activity, related-wallet recurrence, shared-funder clusters and retained liquidity-removal history are exposed as explicit signals;
- verified technical campaign fingerprints can be shown as technical correlations;
- the same projection is available in wallet investigations and canonical token dossiers;
- coordination output has no grade, verdict, same-operator, real-world identity, criminal-group or market-manipulation authority.

Remaining work:

- improve deployer/creator/holder/LP-role recurrence coverage across launches;
- add stronger coordinated timing signatures with replayable evidence;
- expose cluster lineage/supersession cleanly in customer investigations;
- enrich cross-token actor histories without collapsing inferred clusters into one person.

A cluster is not automatically one person. Common control/ownership must never be claimed from timing or shared infrastructure alone.

### B. Market-manipulation intelligence

Current state: **partial deterministic engine already implemented; two evidence-scoped screening layers now exist in this branch.**

Existing unified-radar behavior rules already cover:

- abnormal volume/liquidity gaps;
- holder/liquidity pressure;
- creator-sell acceleration;
- dominant-holder first-exit behavior.

Branch expansion:

- bounded recent transaction rows produce rapid round-trip and synchronized opposite-side burst indicators with explicit temporal coverage requirements;
- persisted `token_trade_events` now feed a 24-hour balanced round-trip churn classifier;
- the persisted classifier requires at least 3 buys, 3 sells, 5 SOL gross flow and a net/gross ratio no greater than 0.15 before a wallet is surfaced as a churn candidate;
- the result is attached to canonical token dossiers as `market_manipulation_intelligence`;
- `wash_trading_proven`, `manipulation_claim`, `same_operator_claim` and `verdict_authority` remain false in this v1 layer;
- the deterministic verdict boundary remains unchanged.

Remaining work:

- transaction-counterparty self-flow and circular-flow proof;
- repeated counterparty/flow motifs across venues and assets;
- stronger synchronized multi-wallet entry/exit timing with funding/topology corroboration;
- stronger liquidity add/remove attribution and reserve-delta timelines;
- repeated market-maker/funding-cluster reuse across assets;
- pump/dump trajectory timelines with exact price/volume/liquidity provenance;
- independent replay tests proving false-positive resistance.

Customer output must say **manipulation indicators** unless stronger factual evidence exists.

### C. Durable campaign intelligence

Current state: **strong foundation implemented.**

The repository already has append-oriented campaign-genome evidence, revisioned Global Campaign state, canonical evidence hashes and evidence-anchor lookup/merge logic. Entity-only hints and verdict references are intentionally excluded from automatic campaign matching where they would create unsafe identity convergence.

Remaining work:

- enrich campaign material from new evidence lanes without creating a second graph model;
- expose campaign timelines and evidence quality in customer investigation views;
- connect public-source observations only through explicit evidence references or verified relations;
- preserve ambiguity/fail-closed behavior when several campaigns could match.

### D. Public hype / promotion / community coordination intelligence

Current state: **provenance-safe branch foundation implemented; provider coverage remains incomplete.**

Branch foundation:

- migration `134_public_promotion_evidence.sql` adds an append-only public evidence store;
- canonical public URL/domain provenance is retained;
- public account/handle, external item ID, source reference and observation/publication times are recorded;
- bounded excerpts are fingerprinted rather than treated as verdict text;
- exact evidence observations receive deterministic `KPUB1-*` references;
- repeated public actor, public domain and normalized-claim fingerprints can produce cross-asset **inferred/watch-only** correlation signals;
- public handles are not promoted into on-chain actor identities;
- public-source overlap has no verdict, grade, same-operator, real-world-identity or wrongdoing authority;
- an observation can be represented as Global Campaign evidence without manufacturing a relation anchor;
- canonical token dossiers now expose `public_promotion_intelligence` with confidence/limitations and cross-asset overlap context.

Still required before this becomes a complete customer feature:

- lawful public-source provider adapters for supported platforms;
- verified official project-site/social-account provenance binding;
- source-specific replay/idempotency tests;
- synchronized public-post versus on-chain-event timelines;
- public claim versus on-chain evidence comparison;
- measurable bot-like/coordinated-public-activity indicators with defensible thresholds;
- richer customer UI/report visualization and opted-in notification policy.

Do not scrape or infer private groups/messages that the system is not lawfully authorized to access. Do not call a community criminal or fraudulent merely because it is promotional, repetitive or coordinated.

### E. Liquidity / MEV / execution intelligence

Current state: **partial.**

Remaining work:

- signed/traceable add-remove liquidity attribution;
- reserve and pool-state before/after evidence;
- LP authority relationships;
- route/slippage and sandwich-like execution evidence;
- clear separation between execution risk and attacker attribution.

A withdrawal or MEV-like ordering pattern is not automatically malicious.

### F. Contract and transaction behavior intelligence

Current state: **strong Solana transaction/token foundation; deeper semantics and broader networks still required.**

Remaining work:

- richer instruction/function semantics;
- privilege and upgrade paths;
- hidden/indirect transfer restrictions where observable;
- approval/delegation and authority-change sequences;
- honeypot-like behavior only where executable simulation or verified transaction evidence supports it;
- proxy/implementation and privileged-admin relationships on supported EVM networks;
- malicious interaction patterns with exact transaction evidence.

### G. Scam / impersonation / phishing surface intelligence

Current state: **partial incident/security memory; public provenance lane now has a branch foundation.**

Remaining work:

- verified official-domain and public-account provenance;
- fake mint/contract similarity with evidence;
- impersonation/domain lookalike evidence;
- claim-site and unsafe-instruction observations;
- malicious-address and incident-memory correlation;
- provenance-preserving customer warnings tied to concrete evidence.

### H. Cross-chain / global network intelligence

Current state: **partial contracts/schema and campaign concepts; not complete production evidence coverage.**

Remaining work:

- chain-specific verified adapters;
- bridge deposit/withdrawal correlation;
- wrapped/native asset lineage;
- cross-chain wallet/entity evidence edges;
- repeated campaign movement across chains;
- peel/conversion patterns only when supported by traceable evidence;
- explicit UNKNOWN when a bridge or destination cannot be verified.

"All crypto" is the product mission and coverage target, not a present-tense claim that every chain and venue is already covered. Solana remains the strongest evidence lane while additional networks must pass explicit readiness gates.

## Customer delivery status

The customer should receive an investigation, not raw noise. The target ARVIS view contains:

- what was detected;
- target/project/token/wallet identities;
- linked wallets, pools, contracts and public project surfaces;
- timeline;
- verified facts;
- inferred/watch-only relationships;
- unknown/missing evidence;
- manipulation/coordination indicators;
- repeat-operator history;
- evidence references;
- recommended defensive action;
- signed verdict where the deterministic verdict contract applies.

This branch also adds the first customer-specific Telegram result-delivery path by reusing the existing consented Telegram pairing. That is an initial lane for successful ARVIS radar checks, not yet proof that every asynchronous ARVIS scan surface is covered.

Delivery targets remain the Web3 application, reports, API/webhooks and opted-in Telegram notifications.

## Corrected implementation order

P0 — preserve standalone Web3 identity, evidence authority boundaries and fail-closed behavior.

P1 — harden actor coordination and market-behavior screening with replayable counterparty/circular-flow evidence and false-positive tests.

P2 — complete public promotion/community evidence provider adapters, official provenance bindings, replay/idempotency and public/on-chain timeline correlation.

P3 — expose actor, campaign, market-behavior and public evidence in one customer investigation view without merging identity authority.

P4 — deepen liquidity/MEV attribution, contract/transaction semantics and phishing/impersonation provenance.

P5 — expand verified multi-network adapters and cross-chain campaign movement evidence.

P6 — cover all relevant ARVIS scan/event surfaces with customer-selected Web3 delivery channels, including Telegram, API/webhooks and reports.

## Non-negotiable truth rule

No evidence, no claim. Correlation is not identity. Coordination is not automatically fraud. Promotion is not automatically manipulation. Missing evidence is UNKNOWN, never SAFE.
