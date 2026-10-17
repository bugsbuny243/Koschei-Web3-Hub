# Owner Security incident review candidates — additive UI v1

This frontend depends on the read-only opt-in backend delivered by draft PR #1396. Review and merge **PR #1396 first**, then rebase this stacked PR onto main; never enable the surface against a backend that lacks `GET /api/owner/incidents?suggestions=1`.

## Wiring
- Existing authenticated `/owner-production` HTML adds a single independent mount, `#ownerIncidentReviewMount`, **before** existing `#securityContent` inside `#page-security`.
- New `/js/owner-incident-review-candidates.js` mounts in that separate DOM node. It does not patch `owner-control-center.js`, `owner-command-center-v2.js`, authorization, navigation, incident forms or security renderers.
- A scoped class-attribute observer starts reading the queue only when the owner app and Security tab are active. Other tabs never fetch this API.
- The view is read-only until the owner expands the alert's evidence limitations and explicitly confirms **Open incident**. Creation reuses the existing `POST /api/owner/incidents` and its persisted-alert reference verification.
- The existing canonical incident list is refreshed through the existing Security navigation control; no parallel case store or alternate write path is introduced.
- Strings from alerts are always rendered through DOM `textContent`, never `innerHTML`.
- Missing service, contract mismatches and HTTP errors show `unavailable`; an empty recent-alert window is **not** a safety result.

## Security and continuity
- No deleted/replaced module, no migration, no feature/tier/auth changes, no auto-incident creation or auto-containment.
- This is a small optional panel inside the existing owner-only response workflow. No customer or public exposure.
- No changes to protected `owner-command-center-v2.js`, `owner-unified-radar.js`, `cipher.js` or authentication code.

## Tests
- `node --check koschei/api/public/js/owner-incident-review-candidates.js`
- `node koschei/api/scripts/verify-owner-incident-review-candidates.js` checks activation, no pre-login fetch, no write before inspection, explicit confirmation, existing alert ID forwarding and keeping the existing DOM intact.
- Existing Go/DB/CI checks must still pass for the full repository. A UI fixture does not substitute for production authorization or live incident flow acceptance.

## Review gates
- Stack parent: PR #1396. The frontend PR must not merge first.
- No production deployment until parent merge, required CI, functional owner session acceptance and regression reviews are complete.
