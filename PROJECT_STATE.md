# Koschei Web3 Project State

## CURRENT STATE

- The production Go runtime is intentionally stateless with respect to Koschei application PostgreSQL persistence. `main.go` does not initialize the application database and does not start database-backed job, watchlist, alert, telemetry or webhook workers.
- Neon remains the authentication boundary. `/api/me` can verify authenticated identity without application persistence.
- Home (`/`) and Customer Panel (`/dashboard`) are the only canonical Koschei Web3 product surfaces. Legacy customer product URLs are being reduced to router-owned compatibility redirects.
- Read-only Solana transaction simulation is live in Customer Panel at `#transaction-preflight`. Koschei does not sign, submit, relay or broadcast customer transactions.
- Durable investigation history, watchlist persistence, stored alerts, DB-backed ARVIS customer investigation and customer feedback storage are not live in the current stateless production process.
- The public case registry remains unavailable in production because the stateless process does not open the application database and the optional Drive registry backend is not fully configured.
- The current Polar checkout HTTP route is DB-backed. Therefore secure checkout is not live in the current stateless process and the browser must not present a working purchase action.

## CHANGED

Cleanup stack before this branch:

- PR #1064 (`cleanup/serious-surface-hardening`) retires dangerous or obsolete handler/UI remnants, preserves shared primitives and aligns Customer Panel with stateless runtime truth.
- PR #1067 (`cleanup/two-surface-scan-retirement`) moves live read-only Solana transaction simulation into Customer Panel; retires standalone Scan, Safe Check, Transaction Shield, Security Radar and Launches surfaces; and establishes the V7 two-surface product contract.
- PR #1071 (`hardening/static-surface-allowlist`) replaces generic public FileServer exposure with an explicit static-surface allowlist. Unknown routes become 404, arbitrary forgotten HTML cannot become a product route, and deployment files such as `vercel.json` are not public assets.

On branch `cleanup/dashboard-operational-surfaces`:

- `/reports`, `/reports.html` and `/watchlist`, `/watchlist.html` now redirect to `/dashboard#evidence`.
- `/arvis-chat`, `/arvis-chat.html` now redirect to `/dashboard#intelligence`.
- Retired standalone `reports.html`, `watchlist.html` and `arvis-chat.html`.
- Retired their standalone-only runtimes: `customer-reports-v2.js`, `customer-watchlist-v2.js`, `customer-arvis-chat-v1.js` and `customer-arvis-metaverse-v1.js`.
- Preserved durable-history, watchlist and ARVIS backend routes/handlers/migrations for a future real persistence plane. UI retirement does not delete backend evidence contracts.
- Reworked canonical-history and watchlist acceptance checks so they continue validating server-side scope, entitlement, evidence, migration and worker contracts while requiring the stateless Dashboard truth projection.
- Retired UI-only Customer Operations and ARVIS Customer Chat workflows/verifiers that existed only to keep the removed standalone pages alive.
- Updated visual-equivalence scope to Home and Customer Panel rather than retired product surfaces.
- Corrected Pricing commercial truth: Professional remains the current deployed entitlement contract, but checkout and persistence-backed Professional operations are not presented as live in the stateless deployment.
- Removed the browser `polar-checkout-v1.js` runtime while the checkout route is persistence-backed and unavailable. Backend Polar handler/gate code remains intact for a future correctly persisted billing plane.

## VERIFIED

Parent PR #1071 at head `876afe8201bb18a8805afb17795f8f1e20c5d078`:

- Auth Freeze Guard passes.
- OpenAPI Contract passes.
- Security CI passes.
- CodeQL passes.
- Supply Chain Security passes.
- Watchlist Evidence-State acceptance passes.
- Enterprise API Keys acceptance passes.
- Canonical Investigation History acceptance passes.
- New static allowlist tests pass inside `internal/http`.
- Full Go workflows reach only the pre-existing frozen Neon issuer expectation mismatch; other Go packages pass.
- Public Product Smoke reaches only the existing production `/api/public/cases?limit=100` 503 after other checked public surfaces/assets succeed.

Current branch `cleanup/dashboard-operational-surfaces`:

- Changes are implemented but the full CI matrix has not yet been run on the final branch head. Do not merge this branch until branch-specific regressions are separated from inherited Neon/registry blockers.

## BROKEN / MISSING

- `/api/public/cases?limit=100` still returns `503` in production. The outer API readiness gate returns `{"error":"database unavailable"}` before the registry handler can serve data.
- The database registry cannot work in the stateless process because `main.go` intentionally supplies no application DB handle.
- The Drive registry alternative cannot be enabled truthfully until its service-account credential is configured; the folder ID alone is insufficient.
- Durable customer jobs/history, watchlists, DB-backed ARVIS customer investigation and stored alerts remain unavailable until a real persistence + worker plane is restored.
- `/api/polar/checkout` is `requiresDB`; current production therefore cannot truthfully offer live checkout.
- The full Go Release/API/Operator gates still encounter the pre-existing Neon issuer test/runtime contradiction. Auth is frozen; do not bypass the guard to hide this.
- Some auxiliary customer/account pages still carry legacy CSS/JS and may reference compatibility URLs. They require separate migration rather than blind deletion.
- TradePI agent routes and migrations still share the Web3 repository/deployment, increasing cross-product blast radius.

## NEXT

1. Open the operational-surface cleanup PR stacked on #1071 and run the complete CI matrix.
2. Fix only branch-caused failures. Do not weaken the production case-registry smoke or auth freeze boundary.
3. Audit the remaining account/developer/auxiliary pages for DB-backed controls that are presented as live in the stateless process; either migrate truth into Customer Panel, explicitly mark unavailable, or retire the surface.
4. Continue removing proven orphan legacy CSS/JS only after their last real consumers are migrated.
5. Design a separately scoped read-only evidence persistence/registry plane, or fully configure the verified Drive snapshot backend. Do not return a fake healthy empty registry.
6. Restore customer persistence deliberately when architecture is ready; do not globally reconnect old application DB workers as a shortcut.
7. Isolate TradePI routes/migrations into their own deployment/repository boundary rather than deleting them blindly from Koschei Web3.

## RISKS

- Re-enabling the existing application `DATABASE_URL` globally in the stateless Web3 process could accidentally resurrect old DB-backed workers, jobs, telemetry or automation. Persistence restoration must be deliberately scoped.
- Treating unavailable durable history or monitoring as an empty collection would misrepresent evidence.
- Treating registry unavailability as an empty healthy publication set would violate the evidence-first contract.
- Advertising checkout while its DB-backed route is unavailable would create a commercial-truth failure even if no payment could complete.
- Removing standalone customer UI must never delete the underlying backend evidence contracts needed for a future real persistence plane.
- Branch protection is not enabled on `main`; exact target freshness and CI status must be checked before merge.

## WORK-IN-PROGRESS POLICY

1. Keep cleanup changes stacked and reviewable; do not collapse unrelated risky changes into one opaque merge.
2. Do not reintroduce fake database state, fake jobs, fake evidence or parallel verdict authority.
3. Preserve Solana exact identity and fail-closed evidence semantics.
4. Keep secrets in environment configuration only.
5. Do not add Koschei Lang or Sentinel implementation work to this repository.
