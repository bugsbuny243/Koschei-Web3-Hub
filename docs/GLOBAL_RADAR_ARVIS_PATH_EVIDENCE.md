# Global Radar -> ARVIS Path Evidence Bridge

Date: 2026-09-23

## Purpose

This adapter is the trust boundary between the Global Radar attack-path layer and the existing `koschei.security-evidence/v1` contract.

It does **not** create an ARVIS grade, score, allow/block decision, or signed final verdict.

## Promotion rule

Only an `EVIDENCE_BACKED` attack path may be promoted.

A `CANDIDATE` path contains at least one hypothesis and is rejected by the adapter. This prevents confidence-based correlation from silently becoming ARVIS evidence authority.

## Evidence mapping

```text
Global Entity Graph
      ↓
Global Attack Path
      ↓
[EVIDENCE_BACKED only]
      ↓
koschei.security-evidence/v1
      ↓
ARVIS decision policy
      ↓
signed final verdict (separate authority)
```

The path SHA-256 is preserved as both:

- the SecurityEvidence source digest; and
- the finding evidence digest.

If every segment is VERIFIED, the resulting finding is VERIFIED. If any evidence-backed segment is only OBSERVED, the resulting finding remains OBSERVED.

Multi-network paths use the subject chain `multi-chain`; single-network paths preserve the original network ID.

## Non-goals

- no attacker identity claim;
- no automatic maliciousness label;
- no grade or score generation;
- no promotion of hypothesis confidence;
- no signing authority.

This keeps the Global Radar additive to ARVIS rather than bypassing ARVIS.
