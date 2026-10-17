# Security Center — raw RPC/API JSON source witness (gate 1)

**Status:** Additive, read-only, opt-in library validation. Stacked on draft PR #1399, which depends on #1398 → #1397 → #1396. Neither this module nor its tests declare a completed production RPC-to-verdict link.

## Why
The existing `radaradapter.ValidateEvidence` checks that a collector row declares a native subject, source, height and `RawHash`. A declared hash alone does not prove the response bytes that produced it. The existing `services.AdaptRadarEvidence` projects adapter rows as **OBSERVED**, deliberately withholding verdict authority.

This new `radaradapter.BindRawJSONSourceWitnesses` is a separately testable source-integrity gate. A collector must supply both an `EvidenceRow` and the **actual raw JSON response bytes** received from its provider.

## Fail-closed requirements
- Complete native `SubjectIdentity`, exact network/native-ID binding and unique EvidenceRow IDs.
- `status=observed` only; caller-declared `verified` is rejected, not promoted.
- Strict JSON bytes present (max 2 MiB each), reproducible lowercase SHA-256 exact match against `row.RawHash`.
- Declared height must be canonical positive decimal uint64, but is **not** independently verified against the JSON response's slot or consensus.
- Declared attributes are hashed into a versioned deterministic commitment. This commits to a declaration but does **not** independently prove the attributes are correct.
- A single invalid item returns an error and zero witnesses; no partial proof escapes.

## Exposure and non-authority
The output includes row ID, network, native subject, reported height, observation timestamp, source bytes SHA-256, declared-attributes SHA-256 and a versioned commitment hash. It excludes raw JSON bytes, raw attributes and endpoint address/credentials. The digest of provider identity, not its raw text, participates in the commitment.

Every witness explicitly sets:
- `integrity_status=collector_raw_json_bytes_match_declared_digest_only`
- `height_independently_verified=false`
- `attributes_independently_proven=false`
- `chain_evidence_verified=false`
- `verdict_bound=false`

The existing Solana upgradeable-program adapter now calls this checker on the **actual getAccountInfo response bytes** before returning its observed program-link row. It adds only three observed-only attributes: source integrity scope, SHA-256 commitment, and `source_integrity_chain_verified=false`. A failed binding returns no collector evidence and a limitation. The existing non-atomic getSlot/getAccountInfo limitation remains. No extra RPC is introduced.

This change does **not** touch ARVIS signing/grade rules, owner APIs, alerts, incident metadata or database migrations. It introduces no new background worker, RPC call, UI route or on-chain write.

## Next source integration requirements
For Solana `getTokenSupply`, `getTokenLargestAccounts` and `getMultipleAccounts`:
1. Supply the original RPC JSON bytes (not re-marshaled objects).
2. Parse the **same raw response** for `context.slot`, balance, mint, token-program owner and token-account owner; compare the parsed slot with declared row height.
3. Reconcile holder supply and owner/account roles with exact integer arithmetic; treat non-atomic slots as such.
4. Link resulting proven observations to a precise verdict ID and signing material only after full independent checks; keep `verified=false` otherwise.

For the first live-code-path integration, test `go test ./internal/radaradapter/solana -run TestSolanaProgramCollectorBindsAccountJSONWithoutExtraRPC -count=1`. This is a fixture-backed code-path test, **not** evidence of a successful production RPC call.

Stage 1-3A pre-existing, locally staged EvidenceRows patch files are not silently applied or marked merged. This module is intentionally a small gate that can be reviewed independently before those patches are adapted to current repository state.

## Checks

```sh
cd koschei/api
go test ./internal/radaradapter -run TestRawJSONSourceWitness -count=1
go test -race ./internal/radaradapter -run TestRawJSONSourceWitness -count=1
go test ./... -count=1
go vet ./...
test -z "$(gofmt -l .)"
```

A test with synthetic JSON bytes proves only determinism and rejection behavior, not live Solana RPC connectivity or production provider reliability.
