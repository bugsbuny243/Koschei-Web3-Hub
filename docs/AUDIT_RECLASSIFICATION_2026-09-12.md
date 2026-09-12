# Koschei Web3 Hub — 2026-09-11 audit reclassification

Date: 2026-09-12
Current main baseline: `453dd7cc1a0768f2b86812d7b22deb93c43fca5c`
Previous audit baseline: `b972ffe5276890e604af069c2bc9575be39a6e45`

This document reclassifies findings F01–F14 from the 2026-09-11 repository audit against the current repository and the latest observed production smoke evidence. It does not silently mark a finding complete merely because code or a test file exists.

Status vocabulary:

- `closed-in-code`: the specific audited code path has been corrected in current main; deployment acceptance may still be separate.
- `partial`: meaningful remediation exists but the original product/security boundary is not fully proven closed.
- `open`: current evidence still demonstrates the original product/security problem.
- `needs-acceptance`: implementation exists, but executed acceptance evidence is still required before closure.

## Current production gate

Latest observed status for baseline `19c7dcfa4308ec8711434c811790695ea9c939b0`:

- Railway deployment: success.
- public API transport/health: success.
- public product smoke: failure.
- failing endpoint: `/api/public/cases?limit=100` returned HTTP 503 for all 12 deployment-readiness attempts.

The failure therefore remains a real product-readiness blocker even though the public pages, scan surfaces and transport health returned HTTP 200.

## F01–F14 reclassification

| ID | 2026-09-12 status | Evidence / reason |
| --- | --- | --- |
| F01 auth redirect can carry token to another origin | `closed-in-code` | `sanitizeFrontendRedirect` now rejects backslash plus CR/LF in addition to non-local prefixes. The exact audited `\\` redirect bypass is no longer accepted. |
| F02 malformed JWT header panic | `partial` | auth hardening changes and regression coverage landed; the exact old unchecked path was modified. Full middleware acceptance on deployed auth remains separate. |
| F03 JWKS cache lacks TTL/miss control | `partial` | JWKS cache hardening code and regression tests were added. Treat as implementation remediation until executed auth/JWKS acceptance is recorded for the current baseline. |
| F04 paid access DB unavailable in production | `open / needs deployment evidence` | current code supports separate `ENTITLEMENT_DATABASE_URL` and remains fail-closed without it. Repository support is not proof the live entitlement ledger is configured and verified end-to-end. |
| F05 application-data features not connected to current runtime | `partial` | `APP_DATABASE_URL` and optional `APP_DATABASE_READ_URL` support now exist and are wired into server/job construction. Live configuration and feature-by-feature acceptance are still required. |
| F06 public case registry/detail unavailable | `open` | latest public-product smoke still observed `/api/public/cases?limit=100` as HTTP 503 across 12 attempts. Portable case code exists but production readiness is not closed. |
| F07 all-network target not backed by collectors | `partial` | Ethereum, Base, Arbitrum, Optimism and Bitcoin are now `probe_ready`; Bitcoin parsing/probe and EVM probe paths were added. `probe_ready` is not full historical collector coverage. |
| F08 canonical address identity differs across paths | `partial / needs acceptance` | intelligence canonicalization changes and canonical regression coverage landed. Existing-record compatibility/migration and deployed behavior still require acceptance evidence. |
| F09 frontend calls without backend contracts | `partial` | metadata route support and frontend updates landed, but a complete active-frontend-to-route inventory has not yet been re-executed on current main. |
| F10 plan/payment copy and entitlement contract diverge | `open / not re-proven` | no current evidence yet proves one canonical plan/feature/payment contract across backend and all active customer surfaces. |
| F11 stateless runtime and rate limiter flag mismatch | `closed-in-code / needs acceptance` | sensitive rate-limit logic was changed and stateless-specific tests were added. Current deployment smoke did not identify this as the failing product gate. |
| F12 legacy dangerous auth/cleanup code | `partial` | `neon_auth_only_cleanup.go` was removed and `local_auth.go` was substantially reduced. Remaining legacy candidates still require reference-backed cleanup, not bulk deletion. |
| F13 deploy succeeds while product acceptance fails | `open` | still reproduced: deployment and public API transport are green while `koschei/public-product` is red. |
| F14 ClickHouse ARVIS read key mismatch / scale risk | `partial / needs measured acceptance` | dedicated ARVIS read-index migration and migration code/tests were added. Closure requires measured query/index evidence on representative data, not migration presence alone. |

## Priority now

1. Close F06/F13 together: make the deployed public registry become operational and verifiable, then make `koschei/public-product` green.
2. Verify F04/F05 as a real customer chain: persistence -> entitlement -> paid action -> history/watchlist/webhook/job persistence.
3. Re-run a current active frontend/API inventory for F09 and unify plan/payment/entitlement semantics for F10.
4. Record measured acceptance for F03/F08/F11/F14 before calling them closed.
5. Keep F07 honest: `probe_ready` may be shipped as probe capability, but must not be described as full multi-network collection.

## Agent evidence update

PR #1122 was reviewed and merged as `453dd7cc1a0768f2b86812d7b22deb93c43fca5c`. It adds signed multi-hop delegation evidence with fail-closed checks for signer mismatch, broken parent hash, intent substitution, disconnected subject paths and cycles. It explicitly does not evaluate authority semantics and cannot manufacture authorization, enforcement, execution or effect evidence.

Open pull requests after the merge: 0.

## Closure rule

A finding may move from `partial`/`needs-acceptance` to closed only when the candidate implementation, executed acceptance result and remaining boundary are recorded together. Production status must not be inferred from repository presence alone.
