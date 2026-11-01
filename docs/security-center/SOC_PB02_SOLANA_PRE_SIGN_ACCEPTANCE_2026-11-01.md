# SOC-PB-02 — Solana pre-sign authority/CPI evidence (2026-11-01)

## Connected route and evidence

The already routed Solana Transaction Guard preflight endpoints accept exact
serialized base64 transaction bytes and perform live RPC simulation. Their v3
authority analyzer decodes outer and inner SPL Token / Token-2022
SetAuthority instructions and InitializePermanentDelegate, with optional
post-simulation state verification. The new soc_pb02_detection field is
computed inside the **actual pre-sign response handler** from the same
decoded transaction, CPI groups and authority evidence that drive Guard's
risk decision. There is no separate manual attack-pattern lookup.

koschei.soc-pb02-pre-sign.v1 exposes independent execution_state,
evidence_state, finding_state, severity, recommended_action,
enforcement_state, report_integrity, bounded matched instructions and
reason codes. Missing simulation, decoded account context, required CPI
or authority evidence returns finding_state=withheld. A normal complete
transaction returns the **narrow** finding
no_pb02_authority_mutation_observed, never SAFE, ALLOW or an overall
transaction security verdict.

A prospective change of SPL owner or mint/freeze authority is an elevated
exposure to review before signing, not proof of malicious intent or
an executed on-chain attack. Revocation is separate. For authority types
whose effect depends on final state (notably 0 through 3), a missing final
state causes WITHHOLD even if the instruction itself decoded successfully.
Inner CPI SetAuthority is marked instruction_source=cpi and cannot
claim complete coverage when required CPI evidence is unavailable.

For a complete, high/critical PB-02 authority mutation, the response handler
refuses to convert an otherwise ALLOW assessment into a permit, even if
the risk-scoring threshold changes in a later code revision. Existing
signed permit issuance and time-bound State Witness requirements remain
separate and are not weakened.

## Scope boundaries

- This is **on-demand pre-sign detection**, not a 24x7 Solana token-program
  subscription. The existing SBX-1 WSS worker observes a limited program
  set and writes only verified mint events; that journal must not be
  described as a comprehensive SPL authority detector.
- enforcement_state=signer_not_connected and
  report_integrity=unsigned_preflight_observation are intentionally
  conservative. Koschei does not control third-party wallet/HSM signing,
  and this signal does not mean a blockchain transaction was stopped.
- No SOC case is automatically created and no evidence is fabricated.
  The general Transaction Guard may emit ordinary alert records on
  non-ALLOW decisions under its existing persisted alert boundary;
  that is not proof of an owner case lifecycle.
- PB-02 in this increment covers decoded SPL SetAuthority and Token-2022
  permanent delegate in the Guard path, **not** all System Assign,
  non-token owner changes, all historic CPI, or automatic program pause.
- A legitimate authority change may match PB-02; behavioral maliciousness
  cannot be inferred from this operation alone.

## Acceptance

The isolated Go tests build real opcode 6 SetAuthority instruction bytes,
exercise the existing authority decoder, exercise real base58-encoded nested
CPI instruction structures, verify normal and missing simulation/CPI cases,
and read the actual HTTP response handler. The assertions protect the
separation between proposed mutation, evidence completeness,
recommendation and external enforcement. GitHub release gates cover these
tests and gofmt on the final commit.

## Missing proof before a 24x7 SOC claim

A separate, durable program-wide Solana subscription must report exact
source freshness, catch-up/backfill gaps, lost queue events and replay;
it must fetch and verify completed transaction bodies and inner CPI
before any incident decision. A live production acceptance with an
authorized operator should measure false positives, missed authority
changes and notification latency. These capabilities are **not**
implemented or claimed by this PR.
