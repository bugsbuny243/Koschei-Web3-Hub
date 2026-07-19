# Koschei Solana Defense Intelligence OS — Phase 9

Phase 9 replaces generic “tests passed” proof language with an owner-approved, versioned reproduction invariant and two independently persisted Railway worker runs.

## Feature gate

Railway web service:

```text
KOSCHEI_DEFENSE_REPRODUCTION_ENABLED=false
KOSCHEI_DEFENSE_WORKER_QUEUE_ENABLED=false
KOSCHEI_DEFENSE_SANDBOX_ENABLED=false
```

Railway Defense Worker:

```text
KOSCHEI_DEFENSE_WORKER_ENABLED=true
KOSCHEI_DEFENSE_SANDBOX_ENABLED=true
```

Enable reproduction only after the worker queue has been accepted.

## Owner endpoint

`GET|POST /api/owner/defense/reproduction`

### 1. Create a versioned invariant

```json
{
  "action": "create_invariant",
  "finding_ref": "KDF1-...",
  "source_artifact_ref": "KDA1-...",
  "invariant_version": "v1.0.0",
  "command": "cargo test",
  "baseline_marker": "KOSCHEI_BASELINE_EXPLOIT_REPRODUCED",
  "patched_marker": "KOSCHEI_PATCHED_EXPLOIT_PATH_BLOCKED",
  "rationale": "Describe exactly what the harness proves and its boundaries."
}
```

The invariant is immutable and bound to one finding, one source artifact, one allowlisted command and two distinct output markers.

### 2. Prepare the pair

```json
{
  "action": "prepare_pair",
  "invariant_ref": "KRI1-...",
  "patch_ref": "KDP1-..."
}
```

Koschei requires an immutable owner patch approval, then queues:

- a baseline worker job with no replacement files
- a patched worker job with the approved replacement files

The Railway web service never executes either job.

### 3. Finalize after both jobs complete

```json
{
  "action": "finalize_pair",
  "invariant_ref": "KRI1-...",
  "patch_ref": "KDP1-...",
  "baseline_job_ref": "KDW1-...",
  "patched_job_ref": "KDW1-..."
}
```

A verified proof requires all of the following:

1. Both worker jobs completed.
2. Both immutable verification records match the same finding, source artifact and invariant command.
3. The baseline run has no patch reference.
4. The patched run is bound to the approved patch reference.
5. Both commands passed.
6. Baseline output contains the exact baseline marker.
7. Patched output contains the exact patched marker.

A compiler success, generic unit-test success or command exit code alone cannot produce a verified proof.

## Persistence

Migration `073_defense_reproduction_invariants.sql` adds:

- immutable versioned reproduction invariants
- immutable paired reproduction runs
- references to both Railway worker jobs
- references to both immutable verification records
- marker-observation booleans
- evidence and limitations
- optional verified proof-of-fix reference

Verified proof remains:

```text
verdict_authority=false
```

It can support the Program Security Lab and customer dossier, but it does not independently alter the signed token-investment verdict.
