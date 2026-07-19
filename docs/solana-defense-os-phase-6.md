# Koschei Solana Defense Intelligence OS — Phase 6

Phase 6 adds read-only deployed-program resolution and source/build-manifest matching without replacing or changing the existing ARVIS and signed Unified Investigation verdict.

## Owner endpoint

`GET|POST /api/owner/defense/deployment`

The route is protected by the existing owner session and database middleware.

### POST request

```json
{
  "program_id": "SOLANA_PROGRAM_ID",
  "network": "solana-mainnet",
  "manifest_artifact_ref": "KDA1-optional"
}
```

The resolver performs only `getAccountInfo` RPC reads. It never signs or sends a transaction.

## Upgradeable programs

For programs owned by `BPFLoaderUpgradeab1e11111111111111111111111`, Koschei:

1. Decodes the Program account.
2. Resolves the ProgramData address.
3. Reads the ProgramData account.
4. Extracts the deployment slot and optional upgrade authority.
5. Separates the loader metadata from the executable bytes.
6. Records both the full account-data bytecode hash and a canonical hash with trailing allocation padding removed.
7. Stores the executable bytes as an immutable `sbpf_bytecode` artifact.

An observed upgrade authority is reported as technical upgrade capacity. It is not evidence of malicious intent.

## Legacy programs

Programs owned by the legacy v1 or v2 BPF loaders are hashed directly from their executable account data. They have no upgrade-authority claim in the Phase 6 snapshot.

## Build manifest matching

An optional immutable `source_manifest` or `sbpf_manifest` can declare one of:

- `compiled_binary_sha256`
- `binary_sha256`
- `full_binary_sha256`
- `canonical_binary_sha256`

Koschei compares the declared hash with the deployed full and canonical bytecode hashes.

Possible results:

- `matched_full_binary`
- `matched_after_zero_padding_normalization`
- `mismatched`
- `invalid_manifest`
- `not_requested`

A hash match proves byte equality with the declared build artifact. It does not independently prove repository ownership, developer identity, source safety or absence of vulnerabilities.

## Evidence boundary

Every deployment snapshot has:

```text
verdict_authority=false
read_only_rpc=true
mainnet_transaction_sent=false
```

The deployment snapshot can support later Program Security Lab analysis, but Phase 6 does not alter an investment grade or signed Unified Investigation verdict.

## Persistence

Migration `070_defense_program_deployments.sql` creates append-only deployment snapshots containing:

- program and network
- loader and ProgramData identity
- RPC observation slot
- deployment slot
- upgrade authority
- full and canonical binary hashes
- immutable bytecode artifact reference
- optional source/build manifest reference
- match status
- evidence references and limitations
- snapshot hash

Updates and deletes are rejected by the existing Defense OS immutability trigger.

## Railway deployment

No Render-specific workflow or environment variable is introduced. GitHub Actions remains CI-only. Production deployment and runtime configuration continue through Railway.
