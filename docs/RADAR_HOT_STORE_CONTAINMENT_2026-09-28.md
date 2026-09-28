# Radar Hot-Store Containment — 2026-09-28

## Scope

This record captures the production storage finding discovered during the full Koschei Web3 repository and runtime audit on 2026-09-28. It is an operational evidence record, not a declaration that historical data is safe to delete.

## Production observation

The PostgreSQL table `security_radar_stream_events` had grown to approximately **97 GB total relation size**:

- heap: approximately **32 GB**
- indexes: approximately **65 GB**
- estimated/live rows: approximately **21–22 million**
- oldest observed row: **2026-09-21**
- newest observed row: **2026-09-28**

The table therefore accumulated this volume in roughly seven days. This is not primarily old dead-row bloat.

Observed distribution showed that nearly all rows were continuous Solana WSS journal observations:

- about 99.8% were `decoded_stream_hint`
- about 97.6% belonged to `pump_sybil_radar`
- the dominant event was `pumpswap_trade_or_liquidity`
- only a small fraction had reached `transaction_enriched_mint`

The production logs also showed repeated `getTransaction` provider timeouts/429 responses while live enrichment attempted to turn raw signatures into mint-scoped evidence.

## Why retention selected zero rows

The retention worker was healthy enough to start and complete its bounded runs, but production was using its default **30-day** window. At the time of observation there were no stream rows older than 30 days, so recent runs correctly selected, archived and deleted zero rows.

Reducing the retention window alone is not a safe storage fix. The current archive ledger resides in PostgreSQL and the external archive sink is disabled by default. Deleting hot rows before an independently verified cold copy would violate the archive-before-delete contract.

## Immediate containment

The continuous WSS journal was disabled in production with:

- `RADAR_STREAM_ENABLED=false`
- `KOSCHEI_AUTO_RADAR_ENABLED=false`
- `KOSCHEI_SOLANA_WATCH_MODE=manual`

Manual ARVIS investigations, customer/API operations and the separately gated selective Pump scheduler are not disabled by this containment.

The repository also adds an explicit production guard: even if a stream switch is accidentally re-enabled, PostgreSQL raw-journal startup remains blocked unless `KOSCHEI_STREAM_POSTGRES_RAW_JOURNAL_ENABLED=true` is deliberately configured.

## Cold-tier status

The repository already contains a ClickHouse stream schema, client and bounded parity copier, but the connected production ClickHouse `koschei_web3` database did not yet contain the required tables when inspected.

The reviewed `ClickHouse Schema Apply` workflow remains an explicit manual production gate. It must not be bypassed merely to reduce PostgreSQL size.

## Required recovery sequence

1. Keep the full raw WSS PostgreSQL journal contained.
2. Apply and verify the reviewed ClickHouse schema through the explicit production schema gate.
3. Copy small, bounded historical windows first and require row-count plus content-fingerprint parity.
4. Expand backfill only after those parity checks remain green.
5. Configure a verified external/cold retention destination.
6. Only then shorten PostgreSQL hot retention and archive/delete old raw journal rows.
7. Reclaim physical PostgreSQL space with an explicitly reviewed maintenance operation after logical deletion is proven safe.
8. Preserve enriched evidence, signed verdicts, published dossiers and audit-critical history according to their independent retention contracts.

## Safety boundary

No production row was deleted as part of this audit or containment. Raw history must not be discarded merely because it is high volume.
