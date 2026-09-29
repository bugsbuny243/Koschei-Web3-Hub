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

Next increments remain: per-network head/cursor lag and gap accounting,
durable replay acceptance under injected failures, verified archive recovery,
and bounded production promotion. Fresh worker cycles alone cannot satisfy
those gates. Web3, Sentinel and Lang retain their existing authority boundaries.
