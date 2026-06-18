# Koschei ARVIS War Architecture

This document defines the target high-throughput architecture for Koschei SBX-1.

The goal is simple: maximize Solana intake speed without slowing the WSS listener down with heavy analysis. The listener must capture data fast, queue it, and let independent ARVIS workers consume and analyze at their own pace.

## 1. Current production mode

Current mode is intentionally simple:

```text
Solana WSS / RPC
  -> Go SBX-1 stream worker
  -> Postgres evidence tables
  -> Security Radar Feed
```

This mode works and must remain available as a safe fallback.

## 2. Target war mode

Target architecture:

```text
Solana WSS Listener (Go)
  -> Redis Streams buffer
  -> ARVIS Go workers
      -> Pump.fun Sybil worker
      -> Raydium Pool Guardian worker
      -> Token authority worker
      -> Wallet-flow / cluster worker
  -> Postgres verdict/evidence store
  -> Live Radar Feed
```

The WSS listener does not perform heavy scoring. It only validates, normalizes, and pushes events into Redis Streams.

## 3. Why Redis Streams

Redis Streams gives Koschei a durable, fast, backpressure-friendly queue.

During high-traffic Solana periods, such as token launch waves, airdrops, or memecoin bursts:

- WSS intake continues.
- Events accumulate in Redis instead of blocking the listener.
- ARVIS workers consume at their own speed.
- The system degrades by queue growth, not by crashing.
- More workers can be added horizontally.

## 4. Queue model

Primary stream:

```text
koschei:sbx1:events
```

Consumer group:

```text
arvis-workers
```

Suggested worker names:

```text
arvis-pump-sybil-01
arvis-raydium-guardian-01
arvis-token-authority-01
arvis-wallet-flow-01
```

Event envelope:

```json
{
  "provider": "alchemy",
  "network": "solana-mainnet",
  "slot": 0,
  "signature": "...",
  "program_id": "...",
  "module_hint": "pump_sybil_radar|raydium_pool_guardian|token_authority|unknown",
  "event_type": "logs_notification|block|transaction|mint_activity|pool_activity",
  "target": "mint_or_pool_or_signature",
  "target_type": "token|pool|signature|unknown",
  "received_at": "RFC3339 timestamp",
  "raw": {}
}
```

## 5. Worker responsibilities

### Pump.fun Sybil worker

Looks for:

- New launch hints.
- Early buyer timing.
- Funding cluster signals.
- Creator-linked buyers.
- Holder concentration.
- Repeated deployer behavior.

Output:

```text
module_id=pump_sybil_radar
risk_index
risk_level
verdict
signals
```

### Raydium Pool Guardian worker

Looks for:

- Pool creation.
- LP concentration.
- Liquidity removal hints.
- Mint/freeze authority state.
- Holder concentration.
- Pool/mint relation.

Output:

```text
module_id=raydium_pool_guardian
risk_index
risk_level
verdict
signals
```

### Token Authority worker

Looks for:

- Active mint authority.
- Active freeze authority.
- Supply anomalies.
- Token account parsing failures.
- Insufficient evidence state.

This worker can feed both Pump and Raydium verdicts as hidden evidence.

### Wallet-flow / cluster worker

Looks for:

- Shared source wallets.
- Synchronized buys.
- Linked wallet clusters.
- Funding fan-out.
- Repeated counterparty relations.

This worker must be evidence-based. If evidence is insufficient, it must return `insufficient_evidence`, not fake certainty.

## 6. Output path

Workers write results into the existing evidence/verdict store:

```text
security_radar_stream_events
security_radar_events
security_radar_verdicts
```

The customer-facing feed still applies the same quality gate:

- Raw stream noise stays hidden.
- Low-confidence partial evidence stays hidden.
- Enriched mint/pool evidence can appear.
- Meaningful medium/high/critical risk appears.
- Green never means guaranteed safe.

## 7. Runtime modes

Koschei must support both modes.

```env
KOSCHEI_QUEUE_MODE=direct
```

Direct mode:

```text
WSS listener -> local/direct processing -> DB
```

```env
KOSCHEI_QUEUE_MODE=redis
REDIS_URL=redis://...
KOSCHEI_ARVIS_WORKERS=4
```

Redis mode:

```text
WSS listener -> Redis Streams -> ARVIS workers -> DB
```

If Redis is unavailable and `KOSCHEI_QUEUE_MODE=direct`, production must not fail.

If Redis is unavailable and `KOSCHEI_QUEUE_MODE=redis`, startup should fail loudly or switch to a clearly logged degraded state based on explicit env:

```env
KOSCHEI_REDIS_REQUIRED=true
```

## 8. Backpressure and reliability

Rules:

- WSS listener must never perform heavy scoring inline.
- Redis stream max length should be capped.
- Workers must acknowledge only after DB write succeeds.
- Failed jobs should stay pending or move to a dead-letter stream.
- Worker concurrency must be controlled by env.
- Duplicate signatures must be deduped at DB/write layer.

Suggested env:

```env
KOSCHEI_REDIS_STREAM=koschei:sbx1:events
KOSCHEI_REDIS_GROUP=arvis-workers
KOSCHEI_REDIS_MAXLEN=50000
KOSCHEI_ARVIS_WORKERS=4
KOSCHEI_ARVIS_BATCH_SIZE=25
KOSCHEI_ARVIS_BLOCK_MS=5000
KOSCHEI_ARVIS_RETRY_LIMIT=3
```

Dead-letter stream:

```text
koschei:sbx1:dead
```

## 9. Feed behavior

The Live Radar Feed must show two separate things:

### Live stream counters

These prove live data is flowing even when visible verdict cards are empty:

- Raw stream events.
- Recognized events.
- Enriched mints.
- Visible verdicts.
- Last event time.
- Last signature.

### Customer verdict cards

These appear only when data quality is strong enough:

- Transaction enriched mint.
- Live RPC evidence.
- Meaningful risk signal.
- Red/high/critical flags.

This prevents the UI from looking fake while still avoiding low-confidence spam.

## 10. Rollout plan

### Phase 1 — Live proof

- Expose SBX-1 stream counters in `/api/v1/radar/feed`.
- Display counters on `/security-radar`.
- Keep low-confidence verdict cards hidden.

### Phase 2 — Optional Redis producer

- Add Redis client dependency.
- Add queue interface.
- Add direct producer and Redis producer.
- WSS listener writes to selected producer based on `KOSCHEI_QUEUE_MODE`.

### Phase 3 — ARVIS workers

- Add Redis consumer group workers.
- Add Pump worker.
- Add Raydium worker.
- Add token authority worker.
- Add wallet-flow worker.

### Phase 4 — Production hardening

- Dead-letter stream.
- Backpressure metrics.
- Worker lag metrics.
- Owner dashboard counters.
- Alert when queue lag exceeds threshold.

## 11. Definition of done

The ARVIS war architecture is production-ready when:

- WSS listener can ingest without blocking.
- Redis stream fills under load instead of crashing the API.
- Workers consume independently.
- DB dedupe prevents duplicate verdict spam.
- Feed shows live counters even when no verdict card appears.
- Customer UI never shows fake/demo data.
- Risk verdicts appear seconds after meaningful evidence is detected.
