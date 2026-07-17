# Production Investigation Acceptance

The acceptance runner executes the real ARVIS full-scan path against 1–10 token mints and evaluates the returned technical report. It does not change the verdict, rules, signing, evidence or tier semantics.

## Target file

Create a local JSON file that is not committed with real token mints:

```json
[
  {"target":"<pump-bonding-curve-mint>","profile":"bonding_curve"},
  {"target":"<dex-traded-mint>","profile":"dex_traded"},
  {"target":"<high-holder-concentration-mint>","profile":"high_concentration"},
  {"target":"<low-holder-concentration-mint>","profile":"low_concentration"},
  {"target":"<observed-creator-sell-mint>","profile":"creator_sell"},
  {"target":"<observed-lp-movement-mint>","profile":"lp_movement"},
  {"target":"<new-token-mint>","profile":"new_token"},
  {"target":"<older-token-mint>","profile":"old_token"},
  {"target":"<low-activity-mint>","profile":"low_activity"},
  {"target":"<high-activity-mint>","profile":"high_activity"}
]
```

The placeholders must be replaced with real production targets. The runner does not include mock token addresses.

## Run

```bash
KOSCHEI_OWNER_SECRET='...' \
KOSCHEI_BASE_URL='https://tradepigloball.co' \
node scripts/run-production-investigation-acceptance.mjs /secure/path/targets.json
```

The runner is strict by default: both `fail` and `partial` produce a non-zero exit code. Temporary diagnostic runs may set `KOSCHEI_ACCEPTANCE_ALLOW_PARTIAL=true`; this must not be used as a release gate.

## Blockers

- target or live-evidence mint mismatch
- schema, ruleset or signed-signature contract failure
- architecture count other than 14
- technical signal count other than 20
- VERIFIED or OBSERVED signal without an evidence reference
- live transaction row without signature, slot, wallet, direction or evidence key
- caller parity mismatch between public, owner and API technical projections
- full investigation that did not request a bounded live transaction window

## Quality floors

Standard traded-token profiles require at least:

- 10 completed collectors
- 10 evidence-producing collectors
- 16 concrete technical signals

Bonding-curve, new-token and low-activity profiles require at least:

- 8 completed collectors
- 8 evidence-producing collectors
- 14 concrete technical signals

A quality-floor miss is `partial`, not a fabricated positive result. Critical evidence gaps are always returned explicitly.
