# Koschei SOC incident handoff JSON v1

**Status:** Additive draft. Depends on #1475 → #1473 → #1472. Not deployed, merged, signed, certified, or evidence-verified.

## Purpose

Owner-authenticated incident responders sometimes need to pass a **consistent snapshot of an operator case and its action history** to a security reviewer. This provides only an integrity checksum of the portable JSON content. It must not be described as an immutable evidence ledger, verified exploit attribution, chain-of-custody certificate, signed attestation, authority to freeze funds or an exchange compliance finding.

## Existing endpoint, opt-in extension

`GET /api/owner/incidents/{uuid}?handoff=1`

This remains inside the **existing owner-only authentication and DB gates**. Without `handoff=1`, the default endpoint is unchanged. All incident POST actions and the ARVIS verdict/evidence system are untouched.

- Uses a PostgreSQL `REPEATABLE READ READ ONLY` transaction for the case and action list.
- Rows are ordered by `created_at ASC, id ASC`.
- Rejects malformed reference arrays and action payloads instead of inventing valid content.
- Includes at most 500 audit actions and rejects histories with 501+ events using HTTP 409 rather than silently producing a truncated “complete” package.
- Rejects oversized payloads (128 KiB per action; 2 MiB of action-payload and descriptive text in aggregate).
- Responds with `no-store` and `nosniff`; missing/unavailable PostgreSQL returns 503. Missing case returns 404.

## Response contract

```json
{
  "ok": true,
  "schema_version": "koschei.soc-incident-handoff.v1",
  "generated_at": "<UTC timestamp>",
  "content": {
    "schema_version": "koschei.soc-incident-handoff.v1",
    "incident": { "id": "<persisted case uuid>", "...": "case metadata and reference lists" },
    "actions": [
      { "id": "<persisted action uuid>", "action_type": "note", "actor": "owner",
        "summary": "operator note", "payload": {"...": "persisted data"},
        "created_at": "<timestamp>" }
    ]
  },
  "content_sha256": "<hex SHA-256 of compact encoding/json marshaled content>",
  "evidence_validation": "not_performed_references_only",
  "actor_verification": "not_performed_owner_operator_labels_only"
}
```

The JSON example is illustrative and contains **no claimed production data**.

### SHA-256 scope

`content_sha256` is computed from the exact compact `encoding/json` serialization of the `content` object (not the response envelope). Recompute it over the returned content using compatible JSON serialization. The `generated_at` envelope timestamp is intentionally not included, and the digest is not a digital signature or independent verification of on-chain events. Its value is only to detect accidental or intentional modifications of the portable record *after* snapshot generation.

The owner SOC read-only page links each case to this opt-in handoff JSON; no automated dispatch, email, external sharing, or uploads are performed.

## Acceptance before promotion

1. Exact-head Go formatting, compilation, unit tests, static analysis, owner-auth regression and full repository release gates.
2. PostgreSQL 17 acceptance on an **isolated** database (with `KOSCHEI_TEST_DATABASE_URL`): audit history included, hash verified, 501+ audit rows rejected; never run fixtures against production.
3. Authorization confirms anonymous and customer requests cannot access either owner page or handoff.
4. Review the amount and sensitivity of case metadata before handing a package to **any third party**. An owner authentication event is not authorization for external redistribution.
5. Real evidence source integrity/retention, provenance validation, analyst identity and any SLA/response commitments require distinct acceptance and cannot be inferred from this endpoint.

No files, services, databases, chain states, or existing ARVIS decisions are deleted or disabled by this feature.
