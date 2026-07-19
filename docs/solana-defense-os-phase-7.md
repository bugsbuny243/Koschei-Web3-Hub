# Koschei Solana Defense Intelligence OS — Phase 7

Phase 7 imports a bounded public GitHub repository snapshot at one exact commit and stores it as an immutable `source_bundle`. It does not execute, build, test or deploy imported source code.

## Railway feature gate

```text
KOSCHEI_DEFENSE_SOURCE_IMPORT_ENABLED=false
```

Keep the gate disabled during the first Railway deployment. Enable it only for owner acceptance after migration 071 has been applied.

GitHub Actions is used only for CI. No Render or GitHub deployment workflow is introduced.

## Owner endpoint

`GET|POST /api/owner/defense/source-import`

### POST request

```json
{
  "program_id": "SOLANA_PROGRAM_ID",
  "network": "solana-mainnet",
  "repository_url": "https://github.com/owner/repository",
  "commit_sha": "40_CHARACTER_GIT_COMMIT_SHA"
}
```

Branch names, tags and abbreviated commit IDs are rejected. The importer requests only:

```text
https://codeload.github.com/owner/repository/zip/COMMIT_SHA
```

## Network and URL controls

- HTTPS is mandatory.
- Only `github.com/owner/repository` is accepted.
- Credentials, query parameters and fragments are rejected.
- Redirects may remain only on `codeload.github.com`.
- The compressed response has a hard byte limit.
- The request has a fixed timeout.

## Archive controls

- Maximum compressed archive size: 8 MiB.
- Maximum expanded size: 20 MiB.
- Maximum archive entries: 1,200.
- Maximum selected source files: 350.
- Maximum individual source file: 256 KiB.
- Maximum selected source bytes: 700 KiB.
- Binary, invalid UTF-8 and NUL-containing files are rejected.
- Symlinks and unsafe paths are rejected.
- `.git`, `target`, `node_modules`, `vendor`, `dist` and `build` content is excluded.

The importer prioritizes Solana and Anchor security material such as:

- `Anchor.toml`
- `Cargo.toml`
- `Cargo.lock`
- Rust source under `programs/` and `src/`
- IDL files
- tests

Lower-priority documentation and client files are included only while the bounded bundle budget remains available.

## Evidence boundary

The resulting source artifact has:

```text
trust_level=observed
verified=false
verdict_authority=false
source_executed=false
production_changed=false
```

A public GitHub archive proves that bytes were fetched from a repository URL at a requested commit endpoint. It does not independently prove:

- repository ownership
- developer identity
- source safety
- source-to-deployment equivalence
- absence of vulnerabilities

Source-to-deployment equivalence is assessed separately by Phase 6 using a build manifest and the deployed sBPF bytecode hash.

## Persistence

Migration `071_defense_source_imports.sql` stores append-only import records containing:

- repository owner and name
- exact commit SHA
- archive SHA-256
- immutable source artifact reference
- selected and skipped file counts
- selected source byte count
- evidence and limitations
- import hash

Updates and deletes are rejected by the existing Defense OS immutability trigger.
