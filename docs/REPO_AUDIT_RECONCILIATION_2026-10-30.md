# Repository audit reconciliation — 2026-10-30

This is a **delta record**, not a declaration that all findings in the historical 2026-10-06 audit are fixed.

## Baseline

The uploaded `repo-denetim-ham-listeler.md` measured `main@9a19ed7` and reported 215 unreachable functions, 353 test-reachable-only functions, 32 unreferenced public assets, and six frontend/backend route mismatches. Those counts **must not** be treated as counts for current `main` without rerunning reachability and frontend reference checks against the exact current commit. This repository now locates the Go runtime and static frontend under `koschei/api/`.

## Verified current-code examples

- `ApplyCrossTokenCreatorHolderTransferRuleV120` is called by `internal/handlers/unified_investigation_report.go`.
- `parsePumpPortalEvent` is called by `internal/services/pumpportal_observable_client.go`. This proves a production call site, **not** provider connectivity or worker activity.
- `ProbeSolanaValidatorTelemetry` and `ProjectSolanaValidatorTelemetryToGlobalRadar` still require explicit production orchestration, bounded provider budgets and operator telemetry. No live rollout is claimed here.
- `EvidenceBackedFinalSecurityRadarVerdict` has tests, but being present does not prove that every customer response uses it. Existing `EvidenceBackedSecurityRadarBundleContext` is wired into the holder intelligence path and evidence gates must remain fail-closed.

## HTTP/static containment in this change

Existing `sensitiveStaticProbePath` already rejects test and fixture directories, test-module suffixes, repository config files and `validation-key.txt`. This change additionally returns 404 for ten legacy/test-only JavaScript entrypoints that no canonical HTML page loads:

- `koschei-modern.js`
- `customer-investigation-ux-v2.js`
- `customer-transaction-preflight-v1.js`
- `evm-authority-desk.js`
- `lp-control-evidence-card.js`
- `premium-evidence-matrix.js`
- `public-scan-request-guard.js`
- `unified-live-evidence-card.js`
- `verdict-card-evidence-refs.js`
- `verdict-card-market-context.js`

Sources remain in Git for tests and future deliberate integrations; they are **not** advertised as working product modules. Canonical `/js/koschei-auth.js`, `/js/customer-universal-address-scan-v1.js`, and `/js/public-solana-scan.js` remain accessible. The existing SDK path stays accessible because SDK compatibility is a separate decision. Unlinked HTML pages are not automatically deleted or blocked: route compatibility needs its own review.

## API route reconciliation

- `/api/public/tool-prices` and `/api/wallet/score` are registered in `internal/http/server.go`. Their presence is not an entitlement or end-to-end execution acceptance.
- `/api/mev/analyze`, `/api/liquidity/analyze`, and `/api/dao/proposal-risk` are intentionally quarantined; their synthetic analysis handlers must **not** be made live to satisfy an old checklist.
- `/api/owner/kosch-access` is not a canonical owner endpoint. The active owner auth/API surface must not silently reintroduce it.
- `frontend_route_quarantine_test.go` scans public HTML and JavaScript to prevent those four retired API paths from reappearing.

## Acceptance and remaining work

The code changes require exact-PR-head Go tests, relevant CI gates, and a new reachability audit before merging. The outstanding A/B functions require individual categorization (prod-intended, compatibility, test helper, explicitly retired), verified runtime call paths, source/UNKNOWN semantics, authorization and budget controls, and tests that exercise **registered routes** rather than invoking the helper in isolation. None of these 568 functions are declared universally production-wired by this static containment patch.

## 2026-10-30 owner Solana validator evidence integration candidate

A deliberately opt-in and operator-triggered path now connects two previously test-only functions: `services.ProbeSolanaValidatorTelemetry` and `services.ProjectSolanaValidatorTelemetryToGlobalRadar`. A successful request reads Solana `getVoteAccounts` via the existing governed RPC client, normalizes a bounded sample of up to 12 validator observations and exposes their evidence references through an owner-authenticated API and console.

- API: `GET /api/owner/radar/solana/validators` (owner-only).
- Private UI: `GET /owner/solana-validators` (owner-only), linked from the owner control center; collection requires an explicit button click.
- Gate: `KOSCHEI_OWNER_SOLANA_VALIDATOR_PROBE_ENABLED=1` **and** a configured `SOLANA_RPC_URL`. Default is disabled.
- Provider budget: a 12-second request timeout, successful sample cached for 10 minutes, failures throttled for 30 seconds and concurrent retries single-slotted. No arbitrary user-selected endpoints.
- Truth boundary: `observed` only; no decentralization score, risk rating, safety verdict, signed decision, durable storage, auto-worker or customer-decision promotion. Missing/invalid provider data fails closed with `unknown`.
- Tests: registered route owner authentication, disabled/unconfigured behavior, read-only fake-RPC roundtrip, cache/failure budget, invalid result rejection and projection evidence semantics.

**Acceptance still required:** exact-head CI/Go race and deployed owner login, production RPC access and final operator observation. The previous A/B ledger items not named above remain to be reviewed; the historical totals must not be restated as the current counts.

## 2026-10-30 ARVIS canonical final authority correction

The historical B-list function `EvidenceBackedFinalSecurityRadarVerdict` was an **unsafe legacy interpretation**, not a missing production scorer. The legacy implementation could select a high-risk-index pump/pool arm when only a different arm supplied verified evidence, or return a compatibility `final_verdict_engine` arm. Neither has canonical signing authority. The production decision contract explicitly delegates final grade authority to `EvaluateUnifiedRadarVerdictV140` / `FinalizeUnifiedRadarVerdictContractForNetwork`.

- The legacy helper is now **unsigned and ungraded**: if no non-final verified evidence exists it reports `insufficient_evidence`; otherwise it returns the existing `evaluate_unified_rules` compatibility placeholder. It cannot choose a legacy arm score or echo a fabricated signature.
- The side-effect-free `EvidenceBackedLegacyFinalFromGatedBundle` consumes an already gated bundle; it must never enrich the bundle or re-query provider RPC.
- The shared investigation report records `final_authority_boundary`: explicit legacy non-authority, evidence-availability status, canonical engine, canonical signed/grade state and immutable-decision assertion. This field is **diagnostic**, not a second verdict, does not alter the canonical verdict and is returned through the existing owner/customer unified report interfaces.
- Regression tests cover forged legacy final arm, unrelated verified arm, unequal risk scores, caller-neutral report output and unchanged canonical verdict.
- No claim of new production signatures, complete RPC evidence, or a finished SOC is made. Canonical collector/signature/persistence acceptance remains independently gated.

## 2026-10-30 Customer live canonical-signature readiness / debit boundary

The production `/api/security/radar/check` previously debited `security_radar_check` credits when any verified ARVIS arm evidence existed, independently of whether the unified customer final was signed. The shared `customerInvestigationStatus` also accepted the boolean `Signed` flag as sufficient authentication when live evidence existed. A flag is not a cryptographic proof.

- `VerifyUnifiedRadarVerdictWithCurrentSigner` authenticates the existing finalized canonical Ed25519 payload via the trusted-key verifier, using only the **public key derived from the currently configured runtime signer**. It binds target, network, rules, evidence, created_at, key ID and canonical payload hash. Missing or invalid signer configuration fails closed; historical keys use the separate trusted registry and are **not** implicitly accepted by the live readiness gate.
- Shared customer `ready` status now requires live verified ARVIS arm evidence **and** successful cryptographic verification. A signed no-grade result is still not a safety approval and missing evidence remains unknown.
- `/api/security/radar/check` consumes paid output **only for ready**; signed=false, forged signatures, missing keys and incomplete evidence return the existing evidence-pending response without debit. No entitlement bypass or new API is introduced.
- Tests verify actual signed result, forged signature, signed-only flag, post-sign tampering of rule facts/target/network, mismatched key and missing signer. Does not itself claim every evidence source is available or that full production end-to-end deployment has been proven.

## 2026-10-30 Customer final verdict durability and receipt linkage

Observed gap: `/api/security/radar/check` saved the ARVIS evidence-arm bundle but did not call the existing unified verdict persistence path for the canonical signed final. The owner scan already called `persistUnifiedRadarVerdict` and displayed history. Consequently the customer could receive a signed `ready` result and incur a premium debit without a corresponding durable, re-readable unified final in `security_unified_radar_verdicts`.

- The customer token radar now builds its shared canonical envelope **before persistence** to capture the final after the summary/synchronization adapter, then writes that exact final via the existing unified verdict persistence path.
- `customerCanonicalReceiptGate` requires the live Ed25519 verification contract, live verified evidence, a primary-DB `persisted` result, and a re-read history record with exactly matching signed payload hash, signature, signer key ID, deterministic digest, network, target, rule version and verdict. A compact TTL cache index or an unverified Signed flag never counts as a durable signed receipt.
- Only an exact matching durable receipt permits the already-existing premium debit. Missing DB, persistence failure, read-back mismatch and absent signer result in **no charge**; if a final was authenticated but could not be linked, response status is `persistence_pending` with explicit `final_verdict_persistence`, no verified receipt and a diagnostic message.
- The shared report and top-level response expose matching `final_verdict_persistence` and `final_verdict_receipt` metadata. They do not invent new SOC events or claim a full Railway deployment test. The old arm bundle persistence remains separate from unified final persistence.
- Regression tests cover exact positive receipt, missing database, compact index, missing/forged signature, tampered evidence, target/network mismatch, wrong signer key and receipt fields.

This work closes a concrete authenticated customer-to-history gap; live SOC production sink acceptance and end-to-end Railway invocation remain independent requirements.
