# Signed Verdict Schema

Koschei ARVIS uses a stable verdict contract for evidence-backed decisions. In this contract, **`signed: true` has one meaning only: the authenticated verdict payload was verified with Ed25519 against a trusted key selected out of band by `key_id`.**

A SHA-256 digest, deterministic fingerprint, database identity, or “produced by the final engine” flag is not a digital signature.

## Required authentication fields

| Field | Purpose |
| --- | --- |
| `target` | Token, wallet, pool, program, transaction, or claim target bound by the signature. |
| `network` | Network identity bound by the signature, for example `solana-mainnet`. |
| `grade` | A-F grade, or `-` when no grade-changing rule was triggered. |
| `verdict` | Deterministic verdict state. |
| `evidence` | Human-readable evidence statements used by the final contract. |
| `rule_version` | Ruleset version that produced or withheld the grade. |
| `triggered_rules` | Rule bindings that contributed to the decision. |
| `decision_path` | Ordered explanation of how the ruleset reached or withheld the grade. |
| `created_at` | Verdict creation timestamp included in the authenticated payload. |
| `payload_hash` | `sha256:<hex>` recomputed from the canonical authenticated payload. |
| `signature_algorithm` | Must be `ed25519`. |
| `key_id` | Selects a public key from the consumer's trusted registry. |
| `signature` | Canonical unpadded base64url Ed25519 signature. |
| `signed` | Must be `true` only when the cryptographic signature is present and verifiable. |

The verifier **must not trust a public key supplied by the verdict itself**. Trust material is configuration owned by the consumer or deployment control plane.

## Canonical authenticated payload v1

The verifier constructs this exact JSON object in this field order and signs/verifies its UTF-8 JSON bytes:

```json
{
  "schema_version": "koschei.arvis-verdict/v1",
  "target": "SOLANA_TARGET",
  "network": "solana-mainnet",
  "grade": "C",
  "verdict": "hard_trigger",
  "evidence": ["..."],
  "rule_version": "koschei-unified-radar-rules-v1.4.0",
  "triggered_rules": [
    {
      "rule_id": "ARD-C001",
      "evidence_status": "verified",
      "title": "Creator reuse",
      "tier": "compounding",
      "grade_effect": "compounding_input",
      "grade_cap": "",
      "count": 1,
      "summary": "...",
      "evidence_keys": ["..."],
      "signatures": ["..."]
    }
  ],
  "watch_flags": [],
  "decision_path": ["..."],
  "created_at": "2026-09-20T03:00:00Z"
}
```

### Canonical byte encoding

The canonical object is encoded as compact UTF-8 JSON with no insignificant whitespace. Field order is the order shown above. ARVIS v1 deliberately matches Go `encoding/json` string escaping so producer and verifier compute the same bytes: `<`, `>`, and `&` are escaped as `\\u003c`, `\\u003e`, and `\\u0026`; U+2028 and U+2029 are escaped as `\\u2028` and `\\u2029`. Canonical rule strings and string-array items are trimmed before authentication.

Rule-array order is the deterministic order emitted by the producer and is preserved by the verifier. Consumers implementing their own verifier should test against `oss/verifier/typescript/testdata/go-producer-vector.json` to catch byte-level drift.

Free-form diagnostic `facts` are intentionally not signing authority in v1. They may be displayed as diagnostics, but changing them does not change the authenticated decision. If a fact must affect a customer decision, it must first be projected into an authenticated rule/evidence field.

`payload_hash` is SHA-256 of those canonical bytes. The Ed25519 signature is over the same canonical bytes, not over caller-provided `payload_hash` text.

## Verification order

1. Validate the required verdict shape.
2. Resolve `key_id` from an out-of-band trusted key registry.
3. Rebuild the canonical authenticated payload.
4. Recompute and compare `payload_hash`.
5. Decode the signature and trusted public key using canonical unpadded base64url.
6. Verify Ed25519 over the canonical payload bytes.
7. Return valid only when every step succeeds.

Unknown keys, malformed keys, malformed signatures, payload mutations, and hash mismatches fail closed.

## No-grade rule

A cryptographically valid contract may contain `"grade": "-"` when no grade-changing rule was triggered. That state must be explained in `decision_path`; it does not imply safety and must never be converted into A.

## Legacy compatibility

Strings such as `koschei-unified:<sha256>` and `koschei-unified-contract:<sha256>` are deterministic fingerprints, **not** Ed25519 signatures. They must not be accepted as a signed verdict by the OSS verifier.
