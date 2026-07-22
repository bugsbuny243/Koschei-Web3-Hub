# Koschei Maximum-Strength Execution Plan

Status: canonical implementation sequence proposal  
Repository baseline: `main@6c095597c3bf55229cdc9bbc0ea9a5be0d0fb10b`  
Actor contract: `ACTOR_INVESTIGATION_ENGINE.md` v1.0  
Unified Radar ruleset: `koschei-unified-radar-rules-v1.0.0`

This document converts the durable repository contracts and the recovered project decisions into one bounded execution order for two product tracks:

1. Koschei Solana Defense Intelligence OS;
2. Koschei ARVIS / Unified Radar.

Conversation history is context, not implementation authority. Merged code, migrations, tests, evidence contracts and phase documents remain authoritative.

## 1. Non-negotiable constitutional boundaries

These rules apply to every future pull request.

- The signed deterministic Koschei investigation verdict is the only verdict authority.
- AI may explain, compare or propose; it cannot create evidence, change a grade, approve a patch or claim proof-of-fix.
- Defense OS evidence always preserves `verdict_authority=false`.
- `VERIFIED`, `OBSERVED`, `INFERRED`, `UNVERIFIED`, unavailable and not-investigated states remain distinct.
- Missing, malformed, stale, mismatched or incomplete evidence fails closed.
- A serious claim requires exact evidence references, including signature/slot/time/account/program information where applicable.
- No Defense OS path may hold wallet material, sign a transaction, submit a mainnet transaction or deploy a production patch.
- No imported source is executed by the web/API process.
- Recipient-wide or holder-wide unbounded history scans are forbidden. Mint-specific ATA paths, bounded windows and explicit budgets remain mandatory.
- The 14-arm ARVIS contract is preserved. A replacement path must prove behavioral parity, rollback safety and evidence compatibility before any legacy path is removed.
- Tiers may change access and capacity, but not the technical truth of a scan.
- No feature is considered complete merely because code exists. Migration, tests, documentation, evidence boundaries, CI and deployment validation are required.

## 2. Change discipline

Every implementation PR must satisfy all of the following:

1. one bounded capability or one bounded security debt item;
2. explicit in-scope and out-of-scope sections;
3. exact canonical contract references;
4. no unrelated refactor, formatting sweep or dependency expansion;
5. default-off feature gate for new execution, automation or provider-cost paths;
6. immutable evidence schema where the result is part of an investigation record;
7. deterministic identity/hash tests where reproducibility is claimed;
8. negative tests for fail-closed behavior;
9. complete required CI before merge;
10. no production enablement bundled into the implementation PR.

Open PRs are completed or closed before overlapping work begins. Parallel work is allowed only when file ownership and runtime paths do not overlap.

## 3. Current checkpoint

### Defense OS

- Phases 1-11: complete.
- Phase 12A: complete — pinned worker/toolchain evidence and fail-closed ready/blocked execution profiles.
- Phase 12B: complete — deterministic offline harness materialization, migration 079, immutable `materialized_harness` artifact.
- Phase 12C: next — first isolated deterministic LiteSVM command run.

### Unified Radar / ARVIS

The production system already contains:

- 14 evidence arms and deterministic signed verdict ownership;
- token authority, holder concentration, funding/sybil and launch/sniper evidence;
- creator and repeat-actor memory paths;
- Helius Enhanced transaction and DAS enrichment with canonical RPC verification boundaries;
- holder cluster/flow relations, CEX/DEX direction and third-party entity provenance;
- protocol-specific liquidity evidence;
- evidence-first Transaction Guard v2;
- durable tenant-scoped alerts and webhooks;
- canonical investigation jobs shared by owner, customer and automatic Pump discovery;
- PumpPortal discovery and selective high-volume enrichment;
- strict owner/session/proxy/CSP/CORS/rate-limit hardening.

The immediate open Radar work at this baseline is PR #659: verified Raydium Burn & Earn permanent LP custody. It must complete review and required validation before additional overlapping liquidity work.

## 4. Defense OS maximum-strength sequence

### D12C — First isolated deterministic LiteSVM run

Goal: execute one fixed offline LiteSVM command only inside the separate Defense Worker.

Required inputs:

- Phase 12A profile with `readiness_status=ready` and `execution_allowed=true`;
- Phase 12B immutable `materialized_harness` artifact;
- exact worker identity and image digest;
- live re-hash of the allowlisted executable immediately before launch;
- no-network worker profile;
- fixed command, arguments and environment;
- bounded wall time, CPU, memory, scratch space and stdout/stderr.

Required immutable result evidence:

- profile, materialization, source artifact, plan and policy references;
- worker ID and image digest;
- executable path and SHA-256;
- command/environment hash;
- start/completion timestamps, duration, exit code and termination reason;
- bounded stdout/stderr plus hashes;
- input and materialization hashes;
- explicit `network_access=false`;
- explicit `mainnet_transaction_sent=false`;
- explicit `verdict_authority=false`.

Non-claims:

- no exploitability;
- no real-world reachability;
- no asset impact;
- no proof-of-fix;
- no program-safety claim.

### D13 — Stateful adversarial sequence engine

Start only after D12C is accepted on a deterministic reference corpus.

- use pinned Trident or an explicitly approved equivalent;
- immutable sequence grammar, seeds and corpus;
- bounded state/action depth;
- distinguish environment failure from program failure;
- preserve minimized deterministic failing sequences;
- never convert fuzz output directly into a verdict or exploitability claim.

### D14 — Reachability and asset-impact proof layer

- bind a finding to a concrete instruction path;
- prove required signer/account conditions;
- bind to deployed bytecode evidence;
- measure unauthorized state or asset delta in controlled fixtures;
- keep capability, exploitability, intent and identity separate.

### D15 — Differential patch verification

- exact baseline and patched artifacts;
- identical fixtures, seeds, commands and invariants;
- target failure removed in patched run;
- accepted invariants remain satisfied;
- Phase 9 paired reproduction remains the proof-of-fix authority;
- no automatic production patch application.

### D16 — Continuous program defense

- Deployment Sentinel changes may queue bounded re-analysis;
- deployment changes invalidate inherited assumptions;
- deduplicated tenant-scoped alerts;
- approved regression suites only;
- explicit cost and concurrency ceilings.

### D17 — Human review and release governance

- separation of duties;
- owner/reviewer approval thresholds;
- immutable evidence signing;
- review queues and audit trail;
- technical dossier export;
- documented rollback and incident procedures.

## 5. Unified Radar maximum-strength sequence

Radar work follows the canonical ten-question actor filter. A feature that answers none of those questions is rejected.

### R1 — Close verified liquidity-control evidence gaps

Complete and validate PR #659 first.

Then close remaining protocol-specific evidence gaps without changing grade authority:

- Raydium CPMM and AMM v4 LP supply, burn, supported locker and creator/controller relations;
- PumpSwap pool/vault/control evidence;
- Meteora DAMM v2 permanent-lock evidence;
- Meteora DLMM position-ownership boundary with no unbounded `getProgramAccounts` call;
- explicit liquidity add/remove traces requiring pinned program, decoded pool, compatible vault deltas, signature and slot;
- swap deltas never mislabelled as liquidity removal.

Success condition: every surfaced lock, burn, add or remove claim carries deterministic protocol-specific evidence and an explicit bounded-observation limitation.

### R2 — Creator and persistent repeat-actor memory

Strengthen the durable actor index:

- creator -> created mint relationships with canonical creation verification;
- first funding source and CEX-opaque termination states;
- creator-to-holder direct transfer/funding evidence;
- repeat creator, dominant holder, funding source, first buyer and recipient relations across tokens;
- role, mint, evidence reference and observation time retained independently;
- actor index remains outside raw-scan retention deletion;
- observed recurrence never becomes identity, intent or common-control proof by itself.

Success condition: the reference `yHCx...6PRe` investigation can produce a complete evidence-backed actor dossier with explicit verified/not-verified states.

### R3 — Launch distribution and bounded recent-history policy

Formalize the recovered cost-control decision without weakening canonical evidence:

- Pump/new-launch and high-volume automatic investigations use a documented recent-history ceiling, targeted at seven days where timestamp evidence is available;
- mint-specific ATA history remains the primary recipient-fate path;
- signature/page/parsed-transaction/RPC/time budgets remain explicit fallback ceilings;
- no recipient-wide full wallet history;
- a truncated window is reported as bounded/partial, never as absence of older activity;
- manual deep investigations may use separately authorized larger budgets.

Success condition: the report states the exact time/signature/ATA window used and distinguishes exhausted history from budget truncation.

### R4 — Holder cluster and flow evidence completion

- preserve inbound and outbound target-token context;
- retain token-account endpoints, owner wallets, mint, standard and decimals;
- positively resolved CEX/DEX/entity provenance only;
- CEX and known DEX routes excluded from common-control grouping;
- common recipient, internal transfer and circular-flow evidence remain relationship evidence, not wash-trading verdicts;
- minimum parsed-wallet thresholds prevent false LOW/safe findings;
- provider failures preserve partial evidence and explicit limitations.

Success condition: no holder-flow claim depends on an unlabeled address, inferred identity or missing direction evidence.

### R5 — Transaction intent and pre-signing defense

Build on Transaction Guard v2:

- complete route-specific claim, swap, approval and token-account lifecycle semantics;
- exact signer/writable/account-owner/program evidence;
- Token and Token-2022 raw account verification;
- deterministic `allow`, `warn`, `block` or `withhold` contracts;
- incomplete/provider-unavailable evidence remains `withhold`;
- no custody and no transaction submission;
- signed alerts transport decisions but never create them.

Success condition: the same canonical evidence packet produces the same decision across public, owner and developer surfaces.

### R6 — MEV and manipulation evidence

MEV/sandwich and market-manipulation claims are enabled only after route-specific evidence exists:

- exact route and pool context;
- transaction ordering and slot evidence;
- pre/post reserve and account-state observations;
- quoted/actual output and slippage evidence;
- repeated swap/consideration/return-flow evidence before wash/self-flow language;
- deterministic behavior rules, not model opinion;
- unresolved observations stay watch-only.

Success condition: no sandwich or wash-trading claim is produced from program identity, circular transfers or timing alone.

### R7 — Cross-chain and external-entity evidence

This remains after Solana-native evidence reaches strong-evidence status.

- verified bridge event ingestion;
- chain, bridge, source/destination, amount, asset and transaction references;
- mixer, peel-chain, stablecoin-conversion and CEX/OTC schemas before claims;
- external labels retain third-party provenance;
- no cross-chain criminal-pattern claim from schema-only data.

### R8 — Radar operations, reliability and cost control

- one canonical investigation worker for owner/customer/automatic sources;
- selective PumpPortal discovery and volume gating;
- provider fallback with explicit source attribution;
- cache and batch behavior tested against stale/mismatched evidence;
- shared PostgreSQL rate limits remain authoritative across instances;
- automatic scanners default off unless explicitly enabled;
- owner-unlimited mode remains test-only and provider throttles still apply;
- every automatic worker has cooldown, dedupe, concurrency and cost ceilings;
- queue recovery, stale lease handling and idempotency tested.

Success condition: a worker restart, provider outage or duplicate event cannot fabricate, lose or double-sign an investigation verdict.

### R9 — Verdict and public-contract consistency

Resolve the remaining contract drift without silently breaking clients:

- deterministic grade and triggered rule identifiers remain canonical;
- internal module heuristics are not presented as AI-generated probability;
- legacy `risk_index` fields are explicitly deprecated, compatibility-scoped or removed through a versioned API migration;
- the same ruleset version is reported across UI, API, alert and dossier surfaces;
- route documentation is generated or verified from the actual server boot chain;
- no customer-visible verdict is signed without verified evidence.

## 6. Shared acceptance corpus

Both tracks use a permanent regression corpus.

### Defense OS corpus

- one known-good deterministic LiteSVM harness;
- one toolchain mismatch;
- one worker-image mismatch;
- missing lock and malformed materialization cases;
- network-attempt rejection;
- timeout/output/memory rejection;
- deterministic rematerialization and rerun identity;
- baseline/patched paired reproduction case.

### Radar corpus

- `yHCx...6PRe` creator dossier;
- repeat dominant-holder observation with no direct creator link;
- CEX-opaque funding termination;
- Helius Enhanced unavailable with canonical fallback;
- fewer than three parsed holder wallets;
- known DEX and positively resolved CEX flow cases;
- liquidity add/remove versus swap separation;
- provider outage producing withheld, not safe;
- duplicate automatic event and worker-restart recovery;
- public/owner/API output consistency.

## 7. Required validation for every implementation PR

At minimum:

- PostgreSQL 17 migration chain;
- immutable-record mutation tests when applicable;
- complete `go test ./...`;
- `go vet ./...`;
- Linux build including API, Defense Worker and Defense Sentinel when touched;
- public JavaScript and Turkish-copy contracts when touched;
- gitleaks;
- govulncheck;
- high-severity/high-confidence gosec gate;
- SDK/event-normalizer checks when their contracts change;
- negative/fail-closed tests for the new capability;
- no production deployment or environment enablement in the code PR.

## 8. Immediate order from the current baseline

1. Finish review and validation of PR #659; do not merge while draft or while required checks are incomplete.
2. Deploy and validate migrations 079 and 080 with all new Defense execution gates still disabled.
3. Implement Defense OS D12C as one isolated-worker PR.
4. Add the Radar bounded recent-history/window evidence contract as one non-behavior-changing foundation PR.
5. Complete creator/repeat-actor durable evidence gaps.
6. Complete remaining liquidity and transaction-intent evidence gaps.
7. Add MEV/manipulation claims only after their evidence rows and acceptance cases exist.
8. Begin D13 only after D12C has a deterministic accepted reference corpus.

## 9. Definition of maximum strength

Maximum strength does not mean maximum number of features or maximum number of provider calls.

A capability reaches maximum strength only when:

- its claim is supported by exact, attributable and reproducible evidence;
- unavailable evidence fails closed;
- bounded collection limits are visible;
- deterministic rules own the outcome;
- AI has no verdict authority;
- the result survives retries, provider failures and worker restarts;
- the same technical truth is preserved across owner, customer, API, alert and dossier surfaces;
- tests prevent regression into identity, intent, safety or exploitability overclaims.
