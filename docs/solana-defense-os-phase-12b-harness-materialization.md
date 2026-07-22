# Koschei Solana Defense Intelligence OS — Phase 12B

Phase 12B converts one Phase 12A-ready execution profile and its immutable owner-prepared harness source bundle into a second deterministic, reviewable materialized harness artifact.

It does not compile or execute source. It does not download, resolve or update dependencies.

## Canonical investigation boundary

Applicable `ACTOR_INVESTIGATION_ENGINE.md` sections:

- §1 question 10 — unavailable evidence is not reported as verified;
- §3 — missing locks, files or profile evidence fail closed;
- §4 — every technical claim retains exact artifact and hash references;
- §5 — materialization has `verdict_authority=false`.

Actor ruleset: `v1.0`  
Unified Radar ruleset: `koschei-unified-radar-rules-v1.0.0`

## Feature gate

```text
KOSCHEI_DEFENSE_HARNESS_MATERIALIZATION_ENABLED=false
```

The owner-only web endpoint is disabled by default.

## Endpoint

```text
GET|POST /api/owner/defense/harness-materialization
```

### Materialize a ready profile

```json
{
  "action": "materialize",
  "profile_ref": "KHEP1-..."
}
```

Phase 12B currently accepts only a `litesvm` profile whose Phase 12A state is:

```text
readiness_status=ready
execution_allowed=true
```

A blocked profile is rejected.

## Input bundle contract

The profile must reference an immutable `source_bundle` artifact with:

```json
{
  "artifact_role": "harness",
  "harness_plan_ref": "KHP1-..."
}
```

The bundle must contain:

- a root `Cargo.toml`;
- a root `Cargo.lock` using a supported Cargo lock version;
- a direct LiteSVM dependency in `Cargo.toml`;
- a pinned LiteSVM package record in `Cargo.lock`;
- at least one Rust test under `tests/*.rs`.

The materializer rejects:

- missing or malformed lock evidence;
- Git dependencies;
- path dependencies that escape the immutable bundle;
- unsafe or reserved paths;
- NUL bytes;
- oversized files or bundles;
- unsupported engines.

No network fallback exists. Koschei never runs `cargo update`, `cargo fetch` or dependency resolution in this phase.

## Deterministic normalization

Phase 12B:

1. validates every relative path through the existing Defense OS path boundary;
2. converts CRLF/CR line endings to LF;
3. removes one UTF-8 BOM when present;
4. adds a final newline to non-empty text files;
5. computes raw SHA-256 for every normalized file;
6. computes separate `Cargo.toml` and `Cargo.lock` hashes;
7. generates `koschei/materialization.json` without timestamps or mutable environment data;
8. stores the normalized project as a new immutable `source_bundle` artifact with `artifact_role=materialized_harness`;
9. records an immutable materialization row through migration 079.

Identical profile and source evidence produces the same materialized artifact and materialization reference.

## Generated manifest

`koschei/materialization.json` binds:

- Phase 12A profile ref and hash;
- source harness artifact ref and hash;
- program ID and network;
- engine and fixed command policy;
- normalized source-file paths, sizes and hashes;
- Cargo manifest and lock hashes;
- explicit no-network and no-execution flags.

It intentionally contains no generation timestamp, deployment label or mutable branch/tag identity.

## Persistence

Migration `079_defense_harness_materialization.sql` adds immutable `defense_harness_materializations` records containing:

- profile, source artifact and materialized artifact references;
- complete file manifest;
- file count and byte count;
- Cargo manifest and lock hashes;
- materialized bundle hash;
- evidence references and limitations;
- explicit non-execution and non-authority flags.

## Explicit non-claims

A ready materialization does not establish:

- successful compilation;
- dependency availability in the worker image;
- successful LiteSVM execution;
- invariant success;
- exploitability or reachability;
- asset impact;
- proof-of-fix;
- source-to-deployment equivalence;
- program safety.

Every Phase 12B response preserves:

```text
dependency_resolution=false
source_executed=false
harness_executed=false
mainnet_transaction_sent=false
verdict_authority=false
```

The first isolated command run remains Phase 12C.
