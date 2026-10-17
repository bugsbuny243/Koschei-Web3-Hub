# Premium ARVIS verdict confidence and signature-source truth

**Dependency:** This Draft PR is stacked on #1406 (which depends on #1405 → #1404). No prior PR has yet been merged to main.

## Root cause

The existing `arvis-premium-contract.js` assigned its generic `Confidence` chip with:

```js
first(final.confidence, cluster.confidence, envelope.confidence) || 'unknown'
```

If `final_verdict` had no confidence field and the holder-cluster collector had `confidence='none'` because it lacked evidence, this was displayed as **confidence of the entire signed grade**. In the 2026-10-17 owner investigation this made `GRADE D` / `signed=true` appear beside `Confidence NONE`, which could be mistaken as inconsistent final-verdict math, instead of insufficient *holder cluster* evidence.

The same UI could set `signed=true` when *any* nonempty signature string was present even if the canonical final verdict did not declare itself signed. A browser reading a flag is not itself an independent Ed25519 verification.

## Additive presentation correction

- **Decision confidence** is sourced exclusively from `analysis_summary.decision.confidence`, or if absent, explicit `final_verdict.confidence`. When neither exists, show **UNKNOWN**, not holder-cluster `none` and not fabricated `high`.
- **Holder-cluster confidence** is shown as a separately labeled evidence domain. `none` may be an accurate status for that scope but is not an overall verdict confidence.
- Mark a verdict **reported signed** in this renderer only if `final_verdict.signed===true` **and** the final-verdict signature string is nonempty. No orphan signature or bare signed flag can elevate the UI.
- Display: **“Reported as signed; verify with an out-of-band trusted Ed25519 key. The browser does not independently verify this verdict.”** This module is a presenter, not the OSS verifier or a signer.
- Existing final grade, risk rules, triggered evidence, production verdict payload, account authorization and persistence are unchanged. No new network calls, credentials or DB migrations.

## Acceptance

A dedicated Node test checks a fixture with `GRADE D`, `signed=true`, `holder_cluster.confidence=none`, plus summary confidence-low precedence, explicit final confidence, orphan signature, missing signature and escaped untrusted confidence data. It also runs the existing Token-2022 premium test suite.

**Not claimed:** A DOM badge cannot establish signature validity, an actual verified Ed25519 signature against a trusted public key, risk correctness, source completeness or PYUSD economic safety. This fixes a UI provenance bug, not missing evidence.

Merge the parent Draft stack first and run all required CI after rebase; do not ship incomplete acceptance as production-ready.
