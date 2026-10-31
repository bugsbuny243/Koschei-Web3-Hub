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
- This does not extend the list past the latest 20 receipts or
  assert the absence of older pending cases, complete coverage,
  active SOC staffing, live customer→case acceptance or enforceable
  blockchain intervention.

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
