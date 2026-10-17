# Owner authority addresses — source-discovery display only

**Stack:** PR #1404 backend runtime coverage truth → PR #1405 owner ARVIS coverage parity → this additive Draft PR. Do not merge a child before its parents.

## Evidence that motivated this change

The user-supplied 17 Oct 2026 owner export contains complete base58 Solana addresses for `source_context.mint_authority` and `source_context.freeze_authority` for a PYUSD investigation. The same `source_context.limitations` explicitly states that creator attribution is discovery-only and needs re-verification from canonical RPC. The canonical token authority arm currently exposes the **presence** of certain authorities, but that is not automatically a cryptographically established proof of which account is currently authorized.

Thus the addresses should be visible with their **source boundaries**, rather than hidden, shortened to unusable fragments, or mislabeled as verified owner control.

## Implemented (display only)

A new independent owner-only script `public/js/owner-authority-source-addresses.js` wraps the existing owner radar rendering. It:

- Retrieves only the source-context `mint_authority` and `freeze_authority` from the payload already supplied by the authenticated owner radar UI.
- Renders full-length base58-shaped addresses in an expandable details section alongside the target identity or verdict card.
- Shows an explicit **source-reported, NOT independently verified canonical RPC** label, never a wallet ownership or malicious intent assertion.
- Includes up to two source limitations, if present.
- When an address is missing/invalid, says no source-supported address is available — **never** infers `revoked`, `closed`, safe or malicious.
- Uses DOM `textContent`, never inserts untrusted source text as HTML. Repeated render removes the prior drawer to prevent duplicate outdated source details.

It adds no network requests, backend endpoints, validator signatures, policy enforcement, background jobs or database writes. Existing owner scan core, immutable report, signed verdict, grade, runtime coverage and protected canonical JavaScript files are unchanged.

## Not completed

This presentation is *not* an exact token-2022 extension-authority proof. Permanent-delegate, transfer-hook, transfer-fee and metadata extension authority facts must be bound to their own raw mint account/program verification before they can be called canonical. This change is intentionally limited to the two source-reported basic mint/freeze authorities.

## Acceptance

The Node fake-DOM test `scripts/verify-owner-authority-source-addresses.js` verifies:
- full mint/freeze pubkeys retained;
- source limitation present;
- no duplicate stale drawers;
- malformed/injected values never enter the DOM as code;
- absence is not treated as revoked.

No private owner export, auth token or raw evidence payload is committed.
