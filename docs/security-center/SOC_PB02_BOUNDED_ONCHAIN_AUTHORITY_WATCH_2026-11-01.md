# SOC-PB-02 — bounded Solana on-chain authority observations (2026-11-01)

## Source and behavior

Unlike the existing pre-sign Transaction Guard, this opt-in process reads
**confirmed on-chain transactions** for explicitly configured accounts.
It does **not** turn on the unbounded SPL Token/Token-2022 WSS journal.

Startup: `StartSecurityRadarWatcher` invokes
`StartSOCpb02AuthorityWatchIfEnabled` only if normal background scanning is
allowed. Startup additionally requires:

- `KOSCHEI_SOC_PB02_WATCH_ENABLED=true`
- `KOSCHEI_SOC_PB02_WATCH_ADDRESSES=<comma-separated Solana public keys>`;
  at most four; duplicate/malformed keys fail closed.
- A configured Solana RPC provider.
- Optional `KOSCHEI_SOC_PB02_WATCH_INTERVAL_SECONDS` between 30 and 300
  seconds, default 60 seconds.

Each poll requests at most four pages of 30 confirmed signatures per address,
stopping at the exact previously persisted signature. A missing cursor
**bootstraps prospectively at the current head**; no historical coverage
is inferred. If the cursor cannot be located in that bounded window or the
provider returns partial pages, the watch *does not advance*.

For each new signature, in oldest-first order, `getTransaction(jsonParsed)`
must match the signature, slot, account key and execution status. Outer and
`meta.innerInstructions` are checked for SPL Token and Token-2022 parsed
`setAuthority` and `initializePermanentDelegate`. A matched instruction
must explicitly target the watched **account**, not merely another account
in the same transaction. Unknown/malformed token instructions or missing
inner CPI data make the entire transaction WITHHELD for this scanner.
Missing data, provider failures, reorg/status disagreement and data caps
do not produce an absence finding or a cursor advance.

Matched operations are inserted into a **small, purpose-built PostgreSQL
observation table**, separate from the historical `security_radar_stream_events`
raw journal. The insert and signature cursor transition are in one database
transaction with CAS of the prior signature. Unique identity deduplicates
events across retries. Owner revocations are distinguished from grants,
and high/critical severity denotes *capability exposure*, never verified
malicious intent.

Schema: `141_soc_pb02_bounded_authority_watch.sql`; tables:
`security_soc_pb02_watch_cursors` and
`security_soc_pb02_authority_observations`.

This is a **read-only unsigned observation**, stored with
`enforcement_state=not_connected` and
`report_integrity=unsigned_rpc_observation`.
It does not stop signatures, pause contracts, create signed ARVIS verdict
receipts, or open owner SOC incidents. It does not claim absolute
correctness of the provider's confirmed chain head.

## Scope and safety limits

- No owner watches are configured or turned on by this PR. The production
  stream raw journal containment and archive-before-delete constraints
  remain in place.
- 120 signatures per account per cycle is a *hard safety cap*, not a
  guarantee of completeness. High-volume accounts can become blocked on
  an unresolved gap, which requires a future reviewed paginated replay
  design or manual incident investigation. It is never silently skipped.
- A missing parsed CPI tree prevents claiming complete coverage. Unknown
  custom-program behavior remains outside this detector.
- New addresses start from a signed Solana RPC response, not a verified
  historical attack sequence; this initial source is not independently
  cryptographically authenticated and is recorded as bootstrap_only.
- 24x7 operation requires separately proving provider uptime, restart
  recovery, freshness, retention capacity, false positives and incident
  delivery under representative real network load.

## Executable acceptance

The Go unit corpus checks confirmed transaction signature/slot/account
matching, nested CPI, benign instructions, malformed or missing evidence,
and watch allowlist limits. The mandatory PostgreSQL 17 CI acceptance
uses a dedicated TLS JSON-RPC server and isolated temporary cursor and
observation tables to assert startup bootstrap, later confirmed authority
observation, dedupe, missing CPI fail-closed cursor behavior and truthful
`enforcement_state` and `report_integrity`. The same test is mandatory
in both API Required CI and Release Gates Verification.
