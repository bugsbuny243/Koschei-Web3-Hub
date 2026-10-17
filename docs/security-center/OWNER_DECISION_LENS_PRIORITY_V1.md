# Owner decision lens — backend investor protection takes precedence

**Draft dependency chain:** #1404 canonical collector runtime truth → #1405 owner coverage parity → #1406 source authority address display → this PR. No production merge and no auto-promotion of grade.

## Reproduced from two separate 2026-10-17 owner exports

A PYUSD investigation and a later ANSEM investigation independently displayed this incompatible pair:
- premium lens: `Why REVIEW? No deterministic blocking rule fired`;
- backend canonical verdict: `grade=D`, `verdict=hard_trigger`, `signed=true`;
- backend `investor_protection_decision.decision=AVOID`, `execution_action=BLOCK`, `cleared=false`.

The exports were inspected privately and are **not committed**. The two targets have different hard-cap reasons (verified owner concentration versus verified cross-token launch/holder recurrence). No allegations of criminal intent, personal identity or coordination are derived from either.

## Root and minimal correction

The existing `owner-investigation-ux.js` created the Decision Lens from a local premium-card `Policy` chip (default `review`), without consulting the already-available `investor_protection_decision` when inserting the lens. Another addon, `owner-investor-protection.js`, attempts a *later* DOM reconciliation; that was not consistently reflected in the captured user display.

The first lens paint now:
1. reads the existing server-supplied investor protection decision;
2. gives explicit `AVOID/BLOCK` precedence over local `REVIEW`, without changing the signed grade or server action;
3. renders `NOT_CLEARED/WITHHOLD` and `REVIEW_FIRST` without claiming a safe outcome;
4. when `hard_trigger` exists without a usable server investor action, fails closed to **ACTION UNRESOLVED**;
5. treats contextual `review` as *not proof no blocking rule fired*. Only the explicitly distinct `allow` copy retains that phrase;
6. preserves the confidence chip and signed flag in separate metadata fields; neither becomes chain-evidence verification.

## Scope and validation

The change is confined to the independent, **non-protected** owner investigation-UX addon. The backend investor-protection policy, signed verdict, cryptographic envelope, grade, user/session controls, ARVIS collectors and canonical projection files are **untouched**. Existing late synchronizer remains in place as defense-in-depth.

Node simulated-DOM test verifies the first render for AVOID/BLOCK despite `Policy review`, missing contract with hard-trigger, ambiguous contextual review, distinct explicit allow, withhold, and separation of `SIGNED` versus `Confidence MEDIUM`.

No database migration, new endpoint, new RPC call, deleted file or real owner addresses in test fixtures.
