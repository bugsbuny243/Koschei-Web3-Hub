# Solana WSS journal and gap replay: fail-closed signature storage (2026-11-01)

## Reproduced code defect

The journal worker and independent gap-healer attempted to call
`SecurityRadarStore.InsertStreamEvent` with a Solana signature as the
target. That verified-mint-only API correctly refuses any target lacking
`transaction_enriched_mint` evidence; thus **neither raw journal nor gap
replay could commit the low-trust signature row**. A bootstrap replay anchor
also used that wrong API.

## Remediation

The journal, gap replay and bootstrap anchor now use a dedicated
`InsertSignatureJournalEvent` boundary, while the verified mint store
**continues rejecting** raw signature targets. The new boundary:

- Accepts only Solana mainnet, base58-shaped signatures, nonzero slots,
  valid program identifiers and exact source modes;
- Checks the nested WSS `logsNotification` signature/slot pair or the
  gap-replay RPC provenance markers against the row being written;
- Rejects forged token targets, unsupported source modes, invalid evidence
  quality, incomplete provenance and oversized notifications;
- Persists `target=NULL`, `target_type=signature`, and only a SHA-256
  digest of the original notification, never a fabricated token mint;
- Preserves `raw_stream`, `decoded_stream_hint`,
  `replay_signature_observed`, `replay_bootstrap_anchor` as
  **source observations**; none are final transaction verification;
- Uses the existing DB dedupe index, reporting duplicate rows as not
  newly recovered; and stops gap progress when source validation or
  PostgreSQL persistence fails.

The production slot-gap healer now requires the same explicit
`KOSCHEI_STREAM_POSTGRES_RAW_JOURNAL_ENABLED=true` gate as the WSS
journal. This prevents enabling raw replay writes indirectly through a
different flag.

The isolated PostgreSQL acceptance test checks real WSS and gap-replay
inserts, deduplication, null target, provenance-quality separation and
the healed signature count. Negative tests cover tampered origins.

## Critical operational boundary — do not enable blindly

The connected production radar hot store previously held approximately
21.8 million Solana WSS rows. Raw stream ingestion created documented
large PostgreSQL storage pressure. This remediation **does not change
any production env flags** and is not authorization to restart a
program-wide unbounded journal. The explicit production containment
and existing archive-before-delete policy remain in effect.

These rows demonstrate observation, not validated transaction effects,
malicious intent, cryptographically signed verdicts, owner SOC incidents,
or complete/caught-up live coverage. A journal event still needs successful
`getTransaction` enrichment, independent state verification and an
authorized SOC routing path before becoming an operational security alert.
`raw_event_sha256` verifies a digest of the original received
notification; it is not an independent proof when the source payload
is not retained.

## Remaining blocked operational acceptance

1. Provision a reviewed bounded cold-tier / archive retention destination
   and independent restore acceptance; do not delete data prematurely.
2. Add bounded, opt-in SPL Token/Token-2022 monitoring by watched entity or
   provider-filtered stream; avoid subscribing to every high-volume token
   program signature without a proven rate/storage budget.
3. Demonstrate gap replay, durable checkpoints, and detector latency with
   real authorized Solana signatures, and separately link verified PB-02
   findings into a real SOC incident with deduplication and audit readback.
4. Do not claim 7x24 attack interception or automatic transaction blocking
   from these journal rows.
