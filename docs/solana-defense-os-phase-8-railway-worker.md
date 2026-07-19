# Koschei Solana Defense Intelligence OS — Phase 8

Phase 8 separates source execution from the Railway web service.

```text
Railway Web Service
  -> owner creates bounded job
  -> PostgreSQL durable queue
  -> separate Railway Defense Worker claims job
  -> ephemeral local verification
  -> immutable verification record and worker event trail
```

GitHub Actions remains CI-only. No Render deployment workflow is used.

## Railway services

### Existing web service

Use the normal `Dockerfile` and keep:

```text
KOSCHEI_DEFENSE_SANDBOX_ENABLED=false
KOSCHEI_DEFENSE_WORKER_QUEUE_ENABLED=false
```

After the worker has been deployed and accepted, only the queue gate may be enabled on the web service:

```text
KOSCHEI_DEFENSE_WORKER_QUEUE_ENABLED=true
```

The web service never executes queued source code.

### Defense worker service

Create a second Railway service from the same repository and set its Dockerfile path to:

```text
Dockerfile.defense-worker
```

Required worker values:

```text
DATABASE_URL=the same Neon database used by the web service
KOSCHEI_DEFENSE_WORKER_ENABLED=true
KOSCHEI_DEFENSE_SANDBOX_ENABLED=true
KOSCHEI_DEFENSE_WORKER_ID=railway-defense-1
KOSCHEI_DEFENSE_WORKER_POLL_SECONDS=2
KOSCHEI_DEFENSE_WORKER_JOB_TIMEOUT_SECONDS=900
```

The worker does not listen on an HTTP port. It polls PostgreSQL for bounded owner-created jobs.

Deploy the web service first so migration 072 exists before the worker starts. The worker opens the database without running migrations.

## Owner queue endpoint

`GET|POST /api/owner/defense/worker-jobs`

Example job:

```json
{
  "action": "verify_bundle",
  "source_artifact_ref": "KDA1-...",
  "finding_ref": "optional",
  "patch_ref": "optional",
  "commands": ["cargo test"],
  "replacements": {},
  "max_attempts": 2
}
```

Only the existing fixed verification-command allowlist is accepted. Shell strings, pipes, redirects and arbitrary executables are rejected before queue insertion.

## Queue guarantees

Migration `072_defense_worker_jobs.sql` adds:

- durable queued/running/completed/failed states
- atomic PostgreSQL claiming with `FOR UPDATE SKIP LOCKED`
- worker lease and stale-lease recovery
- bounded attempts
- request and result hashes
- append-only worker event records

A completion is accepted only from the worker that currently owns the lease.

## Security boundary

- No wallet or private key is supplied to the worker.
- The worker cannot change a signed Unified Investigation verdict.
- Worker results use `verdict_authority=false`.
- Every source bundle is materialized in a temporary directory and removed after the job.
- File paths and commands remain allowlisted.
- Production patch application remains outside Koschei.
- The worker service should have restricted network access, bounded CPU/RAM and ephemeral storage in Railway.

The initial worker image includes Rust and Cargo. Solana, Anchor and Trident commands remain `tool_unavailable` until their dedicated toolchain phase is installed and accepted.
