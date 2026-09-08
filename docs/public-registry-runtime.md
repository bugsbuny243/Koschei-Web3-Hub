# Public evidence registry runtime

Status: production foundation. This document describes the current Koschei Web3 public dossier registry boundary and the gates required before a portable Drive-backed reader may become active.

## Security purpose

`GET /api/public/cases` is a revocable discovery projection over explicitly published, integrity-verified immutable dossiers. It is not a generic cache and it is not allowed to invent availability when its evidence source is missing.

The runtime supports an explicit backend selector:

- `database` — primary publication state remains authoritative;
- `drive` — reads a pre-built `koschei-public-case-registry-v1.json` object through checksum-verifying Google Drive archive code.

There is no automatic database-error to Drive fallback. A backend failure remains a service-unavailable result.

## Stateless readiness boundary

The top-level API readiness middleware must not reject `/api/public/cases` before the registry handler can inspect its own backend. The route is therefore database-optional at the generic middleware layer only.

That does **not** mean the registry becomes database-independent by default:

- with the default `database` backend and no primary DB, the handler returns an explicit `503` with `registry_backend=primary_database`;
- with `drive`, the handler requires a valid configured Drive archive and a semantically valid snapshot;
- `/api/public/soc/feed` remains database-required until it receives its own verified portable evidence source.

## Snapshot publisher

`go run ./cmd/public-registry-publisher` builds the portable object from the same primary loader used to enforce publication ledger linkage, publication effective time and immutable dossier bundle verification.

The publisher:

1. opens `DATABASE_READ_URL`, falling back to `DATABASE_URL`;
2. refuses missing publication persistence;
3. loads a bounded snapshot from primary publication state;
4. refuses any truncated snapshot (`uninspected_publications > 0`);
5. refuses invalid publication or publication-ledger rows;
6. serializes `koschei-public-case-registry-v1`;
7. self-validates the serialized snapshot;
8. writes `koschei-public-case-registry-v1.json` to the configured Google Drive archive;
9. reads the newest object back through checksum verification;
10. requires exact byte/hash/readback parity before reporting success.

A successful write emits only safe operational metadata: object name/id, SHA-256, generation time, publication counts, registry status, publication-ledger status and `readback_verified=true`. Database URLs, service-account JSON, customer secrets and private dossier bytes are never logged by the publisher.

## Configuration

Required server-side environment names for the publisher:

```text
DATABASE_READ_URL or DATABASE_URL
GOOGLE_DRIVE_ARCHIVE_FOLDER_ID
GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON
```

Optional bounds:

```text
KOSCHEI_PUBLIC_REGISTRY_SNAPSHOT_MAX_ROWS=10000
KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS=120
```

`KOSCHEI_PUBLIC_REGISTRY_SNAPSHOT_MAX_ROWS` accepts 1..100000. The publisher fails rather than silently truncating source state.

The service account credential is a secret. It must be provisioned directly in the deployment secret store; it must not be pasted into chat, committed to Git, embedded in frontend code, or written to logs.

## Drive cutover gates

Do not set `KOSCHEI_PUBLIC_REGISTRY_BACKEND=drive` merely because the reader code exists. All of the following must be verified on the same candidate deployment:

1. a real snapshot has been produced from the authoritative publication database;
2. Drive upload and checksum readback both pass;
3. the returned registry passes `verify-public-registry-smoke-response.js`;
4. publish and hide/revocation transitions are exercised against the candidate and stale discovery is not accepted as current truth;
5. missing/corrupt Drive objects return `503`, not an empty or safe-looking registry;
6. no secret is exposed by logs or response payloads;
7. rollback keeps the prior authoritative backend available.

Until those gates pass, production must remain fail-closed rather than switch to an unproven snapshot.

## Architectural direction

Drive is an immutable evidence/archive transport, not Koschei Web3's second transactional database. The target product/intelligence architecture remains ClickHouse-native and append-first. Active revocable publication state should ultimately move to the same versioned evidence/state model instead of recreating PostgreSQL-style mutation semantics in another store.

No Koschei Sentinel or Koschei Lang runtime dependency is introduced by this registry path.
