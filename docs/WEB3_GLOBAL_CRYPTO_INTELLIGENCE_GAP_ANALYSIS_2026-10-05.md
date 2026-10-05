# Koschei Web3 Hub — global crypto intelligence gap analysis

Date: 2026-10-05

This audit translates the standalone Web3 product mission into evidence-backed implementation priorities. It does not claim features are production-ready when the repository marks them partial, planned or schema-only.

## Current strong foundation

The existing repository already has strong evidence paths for:

- Solana token intelligence;
- token authority and holder concentration;
- holder/funding/sybil relationships;
- launch distribution and sniper timing;
- transaction intent and program-relation analysis;
- evidence graph and deterministic verdict concepts.

These should remain the base rather than being replaced by a new umbrella product.

## Current partial areas

The repository already describes these as partial evidence paths and they need completion:

1. **Creator / repeat-operator memory**
   - Persist durable actor-index history.
   - Correlate deployer, creator, funding, dominant-holder, LP and exit behavior across launches.
   - Keep inferred links watch-only until direct evidence exists.

2. **Liquidity manipulation / drain attribution**
   - Attach signed add/remove events, reserve deltas, LP authority and actor relationships.
   - Detect coordinated liquidity movement and exit patterns without treating every withdrawal as malicious.

3. **MEV / sandwich intelligence**
   - Attach route, slippage and pool-state before/after evidence.
   - Separate execution-risk analysis from unsupported attacker attribution.

4. **Watch intelligence**
   - Make observations durable across time and targets.
   - Promote only independently verified evidence into customer claims.

## Planned or missing areas required by the product mission

### A. Market-manipulation intelligence

Current state: planned evidence path.

Required evidence lanes:

- self-flow / circular-flow detection;
- wash-like trading patterns;
- coordinated entry and exit timing;
- artificial volume versus liquidity gaps;
- repeated market-maker or funding-cluster reuse;
- dominant-wallet pressure and synchronized selling;
- pump/dump trajectory evidence;
- price/volume/liquidity anomaly timelines with provenance.

Output must say "manipulation indicators" unless the evidence supports a stronger factual claim.

### B. Multi-wallet and operator-group intelligence

Current state: strong sybil/funding foundation, incomplete actor memory.

Required additions:

- wallet-cluster lineage across multiple tokens;
- shared-funder and common-recipient recurrence;
- deployer/creator/holder/LP-role reuse;
- coordinated wallet timing signatures;
- cluster persistence and supersession history;
- campaign-level graph views spanning launches, wallets, pools and public project identities.

A cluster is not automatically one person. Common control/ownership must never be claimed from timing alone.

### C. Public hype / promotion / community coordination intelligence

Current state: project/metadata and social metadata surfaces exist, but no complete verified campaign-evidence pipeline is established.

Required additions:

- public project-site and public social-account provenance;
- time-bound public promotion observations;
- repeated promoter/project relationships;
- synchronized public-post / on-chain event timelines;
- public claim versus on-chain evidence comparison;
- bot-like or coordinated-public-activity indicators only when measurable evidence exists;
- campaign graph linking public identities, projects, wallets and launches with confidence labels.

Do not scrape or infer private groups/messages that the system is not lawfully authorized to access. Do not call a community criminal or fraudulent merely because it is promotional or coordinated.

### D. Contract and transaction behavior intelligence

Required additions beyond current token/transaction foundations:

- richer instruction/function semantics;
- privilege and upgrade paths;
- hidden/indirect transfer restrictions where observable;
- approval/delegation and authority-change sequences;
- honeypot-like behavior evidence where executable simulation or verified transaction evidence supports it;
- proxy/implementation and privileged-admin relationships on supported EVM networks;
- malicious interaction patterns with exact transaction evidence.

### E. Cross-chain intelligence

Current state: schema-only / no production evidence arm for serious cross-chain claims.

Required additions:

- chain-specific verified adapters;
- bridge deposit/withdrawal correlation;
- wrapped/native asset lineage;
- cross-chain wallet/entity graph edges;
- repeated campaign movement across chains;
- peel-chain and conversion patterns only when supported by traceable evidence;
- explicit UNKNOWN when a bridge or destination cannot be verified.

### F. Scam / impersonation / phishing surface intelligence

Required additions:

- verified official-domain and public-account provenance;
- fake mint/contract similarity and impersonation evidence;
- claim-site and unsafe-instruction evidence;
- malicious-address and incident-memory correlation;
- provenance-preserving URL/domain observations;
- customer warning language tied to concrete evidence.

### G. Global network coverage

The product target is multi-network, but each network must have its own evidence adapter. "All crypto" is the long-term scope, not a claim that every chain is currently covered.

Priority should be determined by customer value and evidence quality, with Solana preserved as the strongest lane while adding EVM and other chain adapters behind explicit readiness gates.

## Customer output target

A customer should not receive raw noise. ARVIS should turn evidence into an investigation view containing:

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

Delivery targets include the Web3 application, reports, API/webhooks and opted-in Telegram notifications.

## Implementation order

P0 — preserve evidence/authority contracts and standalone Web3 product identity.

P1 — complete creator/repeat-operator memory and liquidity attribution.

P2 — implement deterministic market-manipulation evidence rules and durable campaign graphs.

P3 — build public hype/promotion/community evidence ingestion with provenance and confidence boundaries.

P4 — expand contract semantics and multi-network evidence adapters.

P5 — add cross-chain verified movement/campaign correlation.

P6 — unify customer investigation output and opted-in notification delivery across all ARVIS scan types.

## Non-negotiable truth rule

No evidence, no claim. Correlation is not identity. Coordination is not automatically fraud. Promotion is not automatically manipulation. Missing evidence is UNKNOWN, never SAFE.
