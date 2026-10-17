# Owner authoritative decision and verdict persistence truth V1

**Draft stack:** #1404 runtime coverage → #1405 owner display parity → #1406 source authority display → this change. All parents must pass review and be merged/rebased in order. No production rollout is claimed.

## Reproduced discrepancy from two owner HTML exports

Two different source snapshots (PYUSD and ANSEM, 17 Oct 2026) include **the same contradiction**: the authoritative investor protection payload says `decision=AVOID` / `execution_action=BLOCK` / `final_verdict.verdict=hard_trigger` / `grade=D`, but an earlier standalone Decision Lens still says `Why REVIEW?`, `No deterministic blocking rule fired`, and `REVIEW RESULT`. The intended `owner-investor-protection.js` harmonizer already exists, but can miss a render or see stale `OwnerRadarKit.lastScan` during addon composition.

The ANSEM export also contains **`final_verdict_persistence=failed`** alongside `final_verdict.signed=true`. A signature on the in-memory verdict is NOT evidence that the PostgreSQL record was durably written or that the signature was independently verified.

## Additive owner-only response projection guard

A standalone addon, loaded **after** existing owner renderer addons, binds the actual payload passed into `OwnerRadarKit.renderUnified`, `render` or the resolved `scan` call. It exposes a truthful `lastScan` getter and uses the backend `investor_protection_decision` as the only action-label authority.

For the premium card it:
- updates the earlier Decision Lens's heading, explanation, action and `data-policy` to the newest authoritative `decision`, `summary`, `investor_action` and `execution_action`;
- also rechecks when later DOM insertions add the old REVIEW lens;
- creates an idempotent **SIGNED ≠ PERSISTED ≠ INDEPENDENTLY VERIFIED** status section: `persisted` is acknowledged, `failed` or `database_unavailable` receives a critical explicit warning, and absent/unknown states are not silently promoted to persistence.
- never changes grade, rules, signed verdict payload or RPC evidence; never sends writes or new requests.

All untrusted backend text is assigned through DOM `textContent`. The old investor protection section is retained; this addon only reconciles the supplementary earlier lens and displays persistence semantics. Historical case storage is **not repaired** by this UI patch.

## Acceptance and remaining work

New Node fake-DOM regression covers:
- ANSEM-shaped `D/hard_trigger/AVOID/BLOCK` outranking an old `Why REVIEW?` lens, even if `lastScan` previously pointed to a different target;
- `failed` persistence displayed despite `signed=true`;
- follow-up scan correctly updates state without duplicate banners;
- late lens insertion corrected on DOM mutation;
- missing persistence leaves an explicit unverified state.

**Priority backend follow-up:** inspect the PostgreSQL error recorded by `persistUnifiedRadarVerdict`, classify insert/constraint/timeout vs readback failures, and preserve the original signed data until secure replay. Do not call in-memory `signed` equivalent to a retained immutable dossier. Separate Helius transaction-version-1 and context cancellation failures must be traced without raising the allowed Solana transaction version above the reviewed 0 ceiling.

No private MHT payload, credentials, token keys, addresses or production decisions are committed into tests; only invented synthetic decisions are used. No DB migration, deleted file, protected canonical JS edit or automated merge.
