# Owner Decision Lens parity — ANSEM evidence from 2026-10-17

**Scope:** Read-only owner UX correction. Independent draft PR based on main, no dependency on the separate #1404 → #1405 → #1406 coverage and authority-address stack.

## Reproduction

A user-supplied Owner Assistant MHT scan for Solana ANSEM, target `9cRCn9rGT8V2imeM2BaKs13yhMEais3ruM3rPvTGpump`, generated 2026-10-17T17:39:10Z, showed both statements **inside the same DOM premium card**:

- Decision Lens: `Why REVIEW? No deterministic blocking rule fired.` with `REVIEW RESULT` action.
- Backend Investor Protection: `AVOID`, `BLOCK`, `Grade D`, `hard_trigger` with a `VERIFIED URD-C006` evidence record.

Its canonical `investigation_coverage` was **11/14 completed**, **2 evidence pending**, **1 not applicable**, **zero source unavailable**, separately from capability-integration gaps. This export is not a replay of the earlier PYUSD scan and cannot establish cross-day nondeterminism for the same target.

The Investor Protection module already tries to harmonize late-inserted decision lens UI using `OwnerRadarKit.lastScan`. However, Owner Assistant can mount premium cards without the same `OwnerRadarKit.scan` lifecycle; `lastScan` may be missing or from another target. In that case the on-screen, same-card backend protection panel was ignored and the old REVIEW text remained.

## Fix

`owner-investor-protection.js` now prefers a **known backend Investor Protection panel rendered inside the exact same premium card** over the mutable global lastScan value. It extracts the panel's decision, displayed summary, action and execution policy via DOM `textContent` (never raw HTML); only known backend decision kinds are accepted. The panel's source remains the existing backend contract; no alternative grade or risk rule is evaluated by JavaScript.

It also harmonizes when either the decision lens **or the Investor Protection panel** is inserted late. When falling back to the mutable global scan, it additionally requires the report target to match the mint displayed inside the premium card. A stale global scan can no longer attach another token's action to this card. Without any backend decision or on-card panel, the original preexisting conservative behavior is unchanged; no AVOID is inferred.

### Test

A Node simulated-MutationObserver test reproduces the contradictory ANSEM pattern: on-card `AVOID / BLOCK`, previous scan `REVIEW_FIRST` and lens `Why REVIEW?`. It verifies the final lens is `Why AVOID?` with `AVOID TARGET` and `data-policy=block`. It repeats with absent lastScan and reverse insertion order. Existing regression tests continue to run. Dedicated GitHub Action enforces the test.

## Non-goals and evidence limits

- No new RPC calls, transactions, data collection, external lookups, protected owner-controller changes, signed verdict rewrites, grade promotion or database migrations.
- `signed=true` is a recorded signature attribute, not independently verified chain inclusion. Confidence is distinct from the action and grade.
- The exported scan also reported `getSignaturesForAddress: context canceled` and Helius RPC `-32015` for transaction version 1. Those are separate bounded collection problems; **this UI fix does not change the transaction version 0 support boundary** or hide the errors.
- No raw report, private owner snapshot, or real wallet address fixture is committed. Tests use synthetic decisions.

Review, full CI and owner operator acceptance are required before merge.
