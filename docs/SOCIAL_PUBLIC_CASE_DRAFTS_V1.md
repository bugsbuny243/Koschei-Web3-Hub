# Koschei ARVIS — Evidence-first X / Discord draft workflow (v1)

## What this is

An **offline, draft-only** editorial helper for an already publicly released
Koschei immutable dossier. It prepares one X text and one Discord text for
manual review. It does not contain an X API client, Discord webhook, OAuth
token, message scheduler, retry worker, campaign growth engine or network
request. Nothing is posted by this tool.

The canonical Public SOC publication boundary is documented in
`docs/public-soc-v1.md`: an owner explicitly sets
`dossier_publications.status=public`; an immutable export alone is private
by default. The dossier page `/dossier/KD1-...` remains the source of truth.
The draft tool does **not** query PostgreSQL or a live API, so publication
status and evidence integrity must be checked independently before its
input is prepared. A self-attested JSON flag is not a security proof.

## Human verification gate

Before writing the local input, an authorized human reviewer must:

1. Confirm the owner-published case is **currently** discoverable in
   `GET /api/public/cases` and the public dossier URL is accessible.
2. Verify the immutable dossier bundle/signature/evidence and make sure
   claims are explicitly supported by the published public projection.
3. Review all text for private customer identities, unredacted contact data,
   unpublished findings, provider credentials, false accusations and
   unsupported real-world identity/intent inferences.
4. Record the known limitations of the investigation in the input.
5. Decide whether publication is appropriate. A generated draft is never
   approval to post and may need another review immediately before posting.

Do not use unpublished customer investigations or security incident
information as marketing content. No claims of a breach, attacker identity,
financial losses or wrongdoing should be made without public, attributable
and independently reviewed supporting evidence.

## Input and invocation

Prepare a private local `reviewed-case.json` with this structure. Replace
every placeholder with **verified, publicly released** information. The
example below is illustrative and is **not** an existing published incident:

```json
{
  "schema_version": "koschei-social-draft-v1",
  "case_ref": "KD1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "publication_status": "public",
  "owner_publication_checked": true,
  "editorial_review": "approved_for_draft",
  "title": "Public on-chain evidence review",
  "supported_observations": [
    "The reviewed dossier contains an on-chain event timeline."
  ],
  "limitations": [
    "This analysis does not attribute an event to a real-world person."
  ],
  "report_url": "https://tradepigloball.co/dossier/KD1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
```

The URL is constrained to an exact canonical
`https://tradepigloball.co/dossier/<case_ref>` or
`https://www.tradepigloball.co/dossier/<case_ref>` form used by the
current configured deployment. Other deployments require a reviewed code
change to the host allowlist. The tool never follows the URL, so **a valid
URL shape does not prove the report exists or is publicly authorized**.

```sh
node --test scripts/social-draft-generator.test.cjs
node scripts/social-draft-generator.cjs --input reviewed-case.json --dry-run
```

The command prints structured JSON containing:

- `status=draft_only`
- `publication_performed=false`
- `requires_human_posting_approval=true`
- `platforms.x` (at most 280 Unicode characters)
- `platforms.discord` (at most 2000 Unicode characters)

No network calls, local output files, payment operations, account access,
external social actions or background jobs occur. Inputs larger than
16 KiB, private/unapproved cases, unsupported URLs, sensitive tokens,
unexpected control characters and overlength posts fail closed.
The tool does not truncate evidence to satisfy a character budget.

## Responsible community growth

Publish only when there is a genuinely useful, verified public case and a
human has reviewed the text. Prefer precise source links, uncertainty and
methodology over engagement bait. Do not claim to have made a topic trend,
inflate reactions, post duplicate messages to unrelated communities, evade
platform limits or simulate independent endorsements.

## Commercial and production invariants

**Polar remains the only intended payment system and Professional the
only paid package.** This draft workflow has no relationship to sales,
checkout, subscription, API-key entitlements, quotas, pricing, or billing
database schemas. No existing payment or package file changes in this PR.

A future *opt-in* publisher would require separate approval, explicit
scoped credentials, platform policy review, tenant isolation, moderation,
rate limits, audit trails and revocation. It is **not enabled here**.
