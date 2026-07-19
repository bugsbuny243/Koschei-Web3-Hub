# Koschei Solana Defense Intelligence OS — Phase 10

Phase 10 continuously compares immutable Solana program deployment snapshots from a separate Railway sentinel service.

```text
Railway Web Service
  -> owner creates a program monitor
  -> PostgreSQL monitor schedule
  -> Railway Defense Sentinel claims due monitor
  -> read-only Solana RPC deployment resolution
  -> immutable deployment snapshot
  -> meaningful-state comparison
  -> immutable change event when needed
```

GitHub Actions remains CI-only. No Render deployment workflow is used.

## Railway services

### Web service

Keep program execution disabled. Add the owner management gate only after migration 074 is live:

```text
KOSCHEI_DEFENSE_SENTINEL_MANAGEMENT_ENABLED=true
```

### Separate sentinel service

Create another Railway service from the same repository and set its Dockerfile path to:

```text
Dockerfile.defense-sentinel
```

Use:

```text
DATABASE_URL=the same Neon database used by the web service
KOSCHEI_DEFENSE_SENTINEL_ENABLED=true
KOSCHEI_DEFENSE_SENTINEL_ID=railway-sentinel-1
KOSCHEI_DEFENSE_SENTINEL_POLL_SECONDS=10
KOSCHEI_DEFENSE_SENTINEL_CHECK_TIMEOUT_SECONDS=45
SOLANA_RPC_URL=read-only primary RPC
HELIUS_SOLANA_RPC_URL=optional fallback/provider URL
```

The sentinel has no HTTP port and never signs or sends a transaction.

Deploy the Railway web service first so migration 074 exists before the sentinel starts.

## Owner endpoint

`GET|POST /api/owner/defense/sentinel`

### Watch a program

```json
{
  "action": "watch",
  "program_id": "SOLANA_PROGRAM_ID",
  "network": "solana-mainnet",
  "manifest_artifact_ref": "KDA1-optional-build-manifest",
  "interval_seconds": 900
}
```

### Check immediately

```json
{
  "action": "check_now",
  "monitor_ref": "KDM1-..."
}
```

### Disable monitoring

```json
{
  "action": "disable",
  "monitor_ref": "KDM1-..."
}
```

Use `GET ?view=events` to retrieve immutable change events.

## Change classification

The sentinel compares meaningful deployment state and ignores an observation-slot change by itself.

Critical changes:

- deployed canonical bytecode changed
- loader changed
- ProgramData address changed

High changes:

- upgrade authority opened
- upgrade authority changed
- previously matched source/build identity was lost

Informational changes:

- upgrade authority revoked
- source/build match was restored

The first successful check creates a baseline and never creates a change event.

## Evidence boundary

A change event proves that two read-only deployment snapshots differ in a named technical property. It does not establish:

- actor identity
- malicious intent
- exploitation
- wrongdoing
- asset loss

Every event uses:

```text
verdict_authority=false
mainnet_transaction_sent=false
```

Program change events can trigger owner review, a new Program Security Lab analysis and later customer dossier warnings, but they do not independently alter the signed investment verdict.

## Persistence

Migration `074_defense_program_sentinel.sql` adds:

- mutable monitor schedules and leases
- immutable deployment-change events
- previous/current deployment snapshot references
- typed change list, severity and summary
- evidence and limitations
- event hash

The sentinel uses PostgreSQL `FOR UPDATE SKIP LOCKED` claiming so more than one Railway sentinel instance cannot process the same due monitor simultaneously.
