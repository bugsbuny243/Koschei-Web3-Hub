# Web4-Web6 Sentinel acceleration research — 2026-10-05

Status: research input, not a standards claim.

## Fresh signals

### W3C agent protocol work
The W3C AI Agent Protocol Community Group continues active protocol work. Its September 23 minutes identify streaming HTTP authentication, mutual DID authentication, and combining DID identity with OAuth authorization as next-phase discussion topics. These are proposals under Community Group work, not finalized W3C Recommendations. The group has another meeting scheduled for October 7, 2026.

Sentinel consequence: keep identity, authentication, authorization, and protocol adapters separable. Do not bind defensive authority to a single agent protocol or credential form.

### W3C conformance direction
The Agent Conformance and Benchmarking Community Group calls for observable evidence per check, adversarial reference fixtures, and reproducibility metadata including engine version, corpus version, and date. This is Community Group work, not a W3C Recommendation.

Sentinel consequence: every capability claim should be backed by a versioned corpus and reproducible evidence manifest rather than model self-report.

### IETF WIMSE delegation and audit work
The WIMSE document set currently includes work-in-progress drafts for attenuated delegation, capability-bound delegation, authorization audit records, verifier-side delegated-authority evaluation, and authenticated provenance. These are Internet-Drafts and must not be presented as standards.

The current agent audit-record draft keeps authorization decision and independently observed effect distinct, binds records cryptographically, records observation coverage/vantage, and allows a verifier to distinguish an unreadable/indeterminate observation from a negative result.

Sentinel consequence: executor acknowledgement is not containment proof. Defensive action intent, authorization, execution receipt, independent observed effect, observation scope, and agreement must remain separate evidence objects.

## Architecture delta

The existing one-service Koschei Web3 architecture remains correct:

ARVIS deterministic evidence -> Sentinel bounded reasoning -> deterministic policy/delegation gate -> defensive action plan -> authorized executor -> independent effect observation -> recovery/audit evidence.

Rules:
- ARVIS signed deterministic verdict remains authoritative.
- Sentinel model output never grants itself authority.
- The default runtime remains disabled/shadow-only.
- No external target may be acted on merely because Sentinel labels it suspicious.
- Active containment, when later enabled, is restricted to explicitly authorized protected resources.
- Observation failure must not be treated as containment success or containment failure; it is indeterminate until evidence resolves it.
- Production claims require versioned corpus, adversarial fixtures, engine/version/date metadata, CI evidence, isolation evidence, recovery evidence, and live inference evidence.

## Immediate engineering target

Add vendor-neutral defensive control contracts inside the existing `koschei/api/internal/defense` package:
1. evidence-bound security findings;
2. defensive action requests constrained to an authorized tenant/target boundary;
3. deterministic policy decisions;
4. independent effect observations with explicit agreement states;
5. deterministic defensive receipts;
6. shadow-only planning by default, with no mainnet or external-system action path.
