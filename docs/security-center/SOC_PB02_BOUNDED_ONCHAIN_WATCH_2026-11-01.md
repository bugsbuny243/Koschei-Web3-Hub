# SOC-PB-02 — bounded Solana confirmed-transaction authority sensor

## Connected runtime scope

An independently gated background watcher is registered in
`StartSecurityRadarWatcher` and consumes the existing budgeted, SSRF-guarded
Solana RPC client. The collector is **off by default**. To opt in,
operators must set BOTH:

- `KOSCHEI_SOLANA_AUTHORITY_WATCH_ENABLED=true`
- `KOSCHEI_SOLANA_AUTHORITY_WATCH_ADDRESSES=<comma separated 1..3 exact Solana accounts>`

The optional poll interval is
`KOSCHEI_SOLANA_AUTHORITY_WATCH_INTERVAL_SECONDS` (60–900, default 120).
No production environment variable was enabled or changed by this PR.
There are at most 10 signatures per page, 2 pages per cycle, 3 accounts,
with a 45-second per-account budget. This is **polling**, not mempool
interception or continuous unbounded program subscriptions.

For each selected account, `getSignaturesForAddress` discovers confirmed
signatures and `getTransaction(jsonParsed,confirmed)` independently
verifies the exact transaction signature, matching slot, successful
execution metadata, and decoded outer and inner instruction arrays.
Only SPL Token and Token-2022 `setAuthority` or
`initializePermanentDelegate` instructions directly targeting the
watched account can become observations. A normal token transfer or a
different account does not match. Revocations receive a separate medium
severity, rather than a hostile grant finding.

Confirmed is **not finalized**. A matched instruction is an observed,
on-chain successful authority change, not proof of malicious intent,
exchange authorization failure or real-world exploit.

## Atomic evidence and recovery

Migration 141 creates two small independent tables:

- `security_solana_authority_watch_cursors`: one durable watermark and
  coverage status per watched account
- `security_solana_authority_watch_observations`: deduplicated exact
  signature/instruction-scoped observations, with transaction SHA-256
  digest, severity, `enforcement_state=not_connected` and
  `report_integrity=unsigned_rpc_observation`

Evidence inserts and cursor advancement commit in the **same PostgreSQL
transaction** after all matching signatures have been independently
fetched and parsed. No transaction data is stored in the high-volume
raw WSS journal. Any malformed provider result, incomplete CPI coverage,
or RPC failure blocks checkpoint progression.

The bounded catch-up policy is intentionally strict: if a prior
watermark cannot be reached within 20 signatures, status becomes
`gap_unresolved` and the cursor **does not move**. Operators must
resolve the gap; the system must not claim full coverage. The first
bootstrap window, when full, is explicitly marked
`bootstrap_window_only` so the earlier unsampled history is not
silently labelled caught-up. A duplicate observation is not a new attack.

## Acceptance and operational limits

- Confirmed outer and inner SPL `SetAuthority` on an exact watch
- Negative control: ordinary transfer, different account, revocation
- Forged signature/slot, failed transaction, malformed CPI, unsupported
  fields => no admitted event
- PostgreSQL 17 temporary-table acceptance: real unique insert, dedupe,
  durable watermark, unresolved gap, failed RPC without cursor advance
- Required CI stages explicitly execute PostgreSQL acceptance to
  prevent a skipped DB test from looking green

This implementation writes **review candidates**, not automatic
`security_incident_cases`. It does not create signed unified verdict
receipts or pause any token program, and there is no guaranteed event
latency SLA. The next operational acceptance must link eligible
confirmed observations to owner SOC triage with source freshness,
independent finality checks, owner-approved escalation and false
positive measurements. No complete 7x24 SOC claim is made.
