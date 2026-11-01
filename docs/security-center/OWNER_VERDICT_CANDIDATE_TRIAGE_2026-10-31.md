# Signed ARVIS finals → Owner SOC triage queue (2026-10-31)

## Production observation motivating this addition

On 2026-10-31 the connected production Neon project exposed **5**
append-only signed receipts, **0** `security_incident_cases` and **0**
`security_incident_actions`. All five stored canonical payload hashes,
target/network bindings, Ed25519 metadata shape and schema identifiers
matched their PostgreSQL representations. That observation checks record
integrity, **not** independent public-key cryptographic trust; the owner
runtime must verify Ed25519 against the configured historical trusted-key
registry. The lack of cases is a gap in observed operator review, not
proof of a failed database linkage.

## Owner-only workflow

- GET `/api/owner/soc/verdict-candidates` is protected by the **existing
  Owner-only authorization middleware**, must be GET, requires primary
  application persistence, and reads no more than the 20 latest
  `target_kind='token'` immutable receipts. It returns only canonical
  decisions that pass Ed25519 verification using the independently
  operator-configured `KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON`.
- The query records whether an exact signed receipt reference is
  already attached to an Owner SOC incident with matching network and
  target. It never guesses case severity from grades and never creates,
  links, escalates, pauses, blocks or revokes anything.
- GET `/owner/soc-triage`, also owner-only, shows independently verified
  recent signed finals, linked/unlinked state, and a copyable
  `unified-verdict-ledger:<receipt_uuid>:<signature>` reference. The
  human owner uses the existing explicit incident creation/link flow
  for follow-up.
- Both endpoint and page use `no-store` / owner guards. The page
  treats 404/503, missing keys, corrupt signatures and missing schema
  as **unavailable**, not as an empty or healthy queue.
- The default view remains the latest 20 receipts; historical pages are
  available through the scoped cursor contract documented below. Neither
  view asserts the absence of older pending cases, complete coverage,
  active SOC staffing, live customer→case acceptance or enforceable
  blockchain intervention.

## Historical signed receipt browsing (2026-11-01)

The original, unfiltered GET /api/owner/soc/verdict-candidates remains a
read-only 20-newest-token view. The same owner-authenticated route now supports:

- An optional **exact pair** of network and target query parameters,
  selecting one token subject without falling back to another network.
- An optional cursor returned as next_cursor from a preceding response.
  The cursor is an opaque base64url encoding of the exclusive immutable
  (recorded_at,id) keyset boundary. It is not a capability or a signature.
- has_more and next_cursor fields in the unchanged
  koschei.owner-soc-signed-candidates.v1 response. Every page reads at most
  21 rows, independently authenticates at most 20, and returns at most 20.
  The extra row establishes only has_more.
- The private /owner/soc-triage page has an **older signed decisions**
  navigation button. This replaces the displayed page, not the original
  signed payloads. Refresh returns to the latest page.

Example of authenticated owner GET requests:

    /api/owner/soc/verdict-candidates?network=solana-mainnet&target=<exact-mint>
    /api/owner/soc/verdict-candidates?network=solana-mainnet&target=<exact-mint>&cursor=<next_cursor>

Unknown/duplicate parameters, incomplete network/target pairs, malformed
cursors, missing trusted public keys and altered canonical payloads fail closed.
Pages are ordered by (recorded_at DESC,id DESC) so identical timestamps cannot
skip a decision. Old receipts do **not** depend on the current mutable
fingerprint row. They must still pass independent canonical Ed25519 and subject
verification against operator-managed historical public trust keys.

**Boundary:** A page of verified receipts is not a full ledger audit, a
proof of absence of missing or deleted rows, a production customer debit,
a completed owner SOC case, or external WORM retention. has_more=false
means no additional matching rows **in the database at query time**, not
that no earlier decision was ever issued. Historical records that predate
the append-only migration and never had original independently verifiable
canonical bytes cannot be reconstructed or called authenticated history.

Migration 140_unified_verdict_receipt_history_index.sql adds an index for
bounded target-scoped keyset queries. Isolated PostgreSQL acceptance proves an
original receipt remains retrievable after 22 later authenticated finals,
including after the mutable projection signature has been overwritten.

## Gates

- HTTP route test: unauthorized access is hidden, authenticated GET
  requires persistence, and POST is rejected.
- Isolated PostgreSQL receipt + SOC test: verified signed receipt appears
  as unlinked, then linked after explicit case creation, even when the
  legacy mutable projection is changed; missing trust keys and
  forged signature database insert must return 503.
- Owner HTML contract test requires the private API and text-only
  presentation, never injecting remote data as HTML.
- A merge must pass SOC PostgreSQL, API, release, security and all other
  exact-head checks. Production acceptance of an actual customer scan,
  Owner case creation, and re-read still requires authenticated
  operator execution and independent observation.

## Retention blocker (separate)

Railway retention archive export previously halted because its S3 endpoint
was missing/invalid. No connected bucket or durable volume had been
identified in the Railway project. Do not enable unsafe pruning, invent
credentials or use ephemeral filesystem for audit evidence. A durable,
verified S3-compatible destination and restore proof are required.

## Operator acceptance checklist after rollout

An authenticated owner must confirm that the candidate endpoint returns a
successful bounded list, choose a **genuinely applicable** signed receipt,
create or link a real incident through the existing owner workflow, and read
back the action timeline. A non-authenticated observer must never create a
production fixture or claim live case acceptance from stored counts alone.

## Release artifact parity

The new owner-only read route is listed in `productionRouteInventory`
**and** the generated `openapi.yaml`. The release acceptance job rejects
route-inventory drift and stale OpenAPI output; neither check is optional.
