# Radar runtime freshness v1

This first radar reliability increment makes stalled periodic workers visible.
It does not activate collectors, copy/delete production records, change ARVIS
verdicts, or claim continuous chain coverage.

## Contract

`/fabric/security-center/runtime-health` keeps `koschei.runtime-health.v1` and its
existing state vocabulary. Entries add `freshness`, optional `max_age_seconds`
and `fresh_until`. Clients ignoring these additive fields remain compatible.

- `not_monitored`: no periodic success deadline; startup checks alone do not
  establish current availability.
- `awaiting_success`: registered periodic component awaiting its first cycle.
- `fresh`: a successful cycle occurred within the deadline. Zero observations
  are allowed; this does not prove new chain data or complete coverage.
- `stale`: the deadline elapsed. Recorded `live` or `configured` becomes
  `degraded` in the snapshot; failure counters are not fabricated.
- `inactive`: explicitly disabled or stopped.

Repeated registration and failed cycles do not renew the successful-cycle
deadline. Successful cycles recover freshness. Snapshot reads do not mutate
registry history. Restarted components get a new initial deadline.

Head-ingest workers and each configured target use twice the bounded polling
interval plus their 90-second cycle timeout. Background telemetry workers and
each target use twice their bounded interval plus the two-minute cycle timeout.
Event-driven storage writes and occasional reorg recovery are not assigned
periodic deadlines: absence of those events is not evidence of failure.

## Operator surface

`/fabric/security-center` renders the same runtime snapshot, state counts,
freshness, last success, cycle counts and observations. It is a timestamped,
manually refreshed snapshot with `Cache-Control: no-store`. Missing registry
data is explicitly unavailable. Provider error strings are not rendered.

## Validation and remaining gates

Tests exercise silent stalls, exact deadline expiry, repeated registration,
recovery, failed startup, stopped/disabled states, restart and timestamp
isolation. HTTP tests cover missing evidence, disabled components, unmonitored
startup checks, HTML escaping and omission of raw provider errors.

Next increments remain: durable historical gap accounting,
durable replay acceptance under injected failures, verified archive recovery,
and bounded production promotion. Fresh worker cycles alone cannot satisfy
those gates. Web3, Sentinel and Lang retain their existing authority boundaries.


## Per-network ingestion progress

Head ingestion now publishes `ingest` on each target health entry: the observed
provider head, last successfully persisted cursor, pending block distance,
status, and check time. Heights/counts serialize as decimal strings to preserve
uint64 precision. Missing values are omitted and shown as Unknown, never zero.

The cursor advances only after both events and the checkpoint are accepted.
Failures preserve the last confirmed cursor; a later successful cycle reports
recovery. Provider regression does not underflow the backlog counter. Reorg
paths withhold cursor/backlog until the next cycle reads authoritative state.
A new failed cycle replaces prior observations instead of retaining apparent
current coverage. Health success/failure counters remain separate.

Pending distance is not a historical gap inventory. Bootstrap begins at the
observed head, so zero backlog cannot establish full history, completeness,
finality, or a security verdict. Older progress remains timestamped and must be
read alongside worker state and freshness. Process restart requires another
cycle to rebuild this operational projection from durable checkpoints.

Failure-injection coverage proves a checkpoint write error cannot advance the
reported cursor and that the next successful cycle recovers. Additional tests
cover provider regression, missing values, zero backlog serialization, snapshot
isolation, and the operator display of zero versus unknown.
