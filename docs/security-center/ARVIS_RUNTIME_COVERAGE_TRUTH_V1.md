# ARVIS capability runtime coverage truth — diagnostic correction

This patch is based on a user-supplied owner production HTML/MHT snapshot captured on 2026-10-17, but **does not upload that capture** or any private owner access data.

## Reproduced contradiction

The source snapshot has:
- `modules.length=14` (all architectural entries present);
- `investigation_coverage.capability_total=14`;
- `completed=7`, `evidence_producing=7`, `evidence_pending=6`, `not_applicable=1`, `source_unavailable=0`, `insufficient_evidence=0`;
- `critical_gaps=[holder_funding_relations,launch_and_first_buyer_history]`;
- `investigation_coverage.status=partial_investigation`.

Despite this, `capability_integration.capabilities.arvis_14_arm_engine.status=active` in the exported report. The preexisting integration code promoted 14 *present module objects* directly to an active capability, without consulting completed collector coverage. That confuses architecture breadth with runtime evidence.

## Correction

For live scans only, use the canonical `investigation_coverage` aggregate to classify `arvis_14_arm_engine`. `active` requires internally consistent counts, **all applicable arms completed**, no pending/source-unavailable/insufficient statuses or critical gaps, and explicit `status=complete_investigation`. One not-applicable arm does not block completion when every applicable arm has completed. Missing or inconsistent runtime coverage fails closed to `partial`, never to active.

The existing module presence field remains visible, and `evidence_backed` now reflects whether the runtime reports evidence-producing arms rather than whether 14 module objects exist. A readable limitation records the coverage counts when incomplete.

This is **diagnostic presentation only**. It does not change the signed verdict, rule thresholds, RPC timeouts, EvidenceRows, actor state, authorization, source data or operator actions. Stored/preflight non-live status retains its existing behavior.

## Still separate

The exported page's canonical UI panel said `7 complete · 7 partial · 0 N/A` while the backend canonical investigation coverage said `7 completed · 6 pending · 1 not applicable`. The protected canonical JavaScript projector is **not modified** in this PR; presentation parity should receive its own acceptance and design review. Similarly, `context canceled` on a signature read cannot by itself explain all six pending modules, some of which require old launch history or different evidence categories.

A `signed=true` marker on the snapshot must not be mistaken for independent cryptographic verification.

## Tests

An integration regression reproduces exactly 7+6+1 with 14 module objects and proves the capability is `partial`. Other tests cover absent coverage, inconsistent totals, all applicable arms completed (with 1 N/A), and persistent critical gaps.

No database migrations, source rewrites, deleted files or protected UI changes.
