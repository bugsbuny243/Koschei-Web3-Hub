# Koschei Web3 Hub — Final Release Checklist

This checklist defines the acceptance criteria for the final production candidate.

The project must behave like a real Solana security/risk product, not a demo page. Low-confidence stream noise must stay internal. Customer-facing screens must show only verified, enriched, or meaningfully risky evidence.

## 1. Runtime environment

Required production variables:

```env
DATABASE_URL=
SOLANA_RPC_URL=
SOLANA_WSS_URL=
ALCHEMY_API_KEY=
KOSCHEI_AUTO_RADAR_ENABLED=true
KOSCHEI_SOLANA_WATCH_MODE=stream
RADAR_EVENT_BUFFER_SIZE=5000
RADAR_STREAM_STORE_UNKNOWN=false
OWNER_SECRET=
API_KEY_PEPPER=
USER_SESSION_SECRET=
```

Allowed SBX-1 stream enable switches:

```env
RADAR_STREAM_ENABLED=true
KOSCHEI_AUTO_RADAR_ENABLED=true
KOSCHEI_SOLANA_WATCH_MODE=stream
```

If `SOLANA_WSS_URL` is missing, SBX-1 may derive WSS from `SOLANA_RPC_URL` or `ALCHEMY_API_KEY`, but production should still set the explicit WSS URL.

## 2. Deploy acceptance

Production deploy must show the service as healthy in Railway.

Expected log line:

```text
security radar SBX-1 WSS collector started
```

Expected startup behavior:

- Database connects.
- Migrations apply or skip safely.
- API listens on the configured port.
- SBX-1 collector starts when enabled.
- Collector reconnects without crashing on transient WSS failures.

A `Vercel` status failure is non-blocking when the live production service is Railway-backed. Domain routing should still be checked manually.

## 3. SBX-1 data-quality gate

Customer feed must not show raw low-confidence stream noise.

Hidden/internal-only events:

- `sbx1_stream` events with risk index `<= 20`.
- Events with only `partial_rpc_evidence`.
- Events where the target is only a transaction signature and no mint/pool target was enriched.
- Events without `transaction_enriched_mint`, `live_rpc_evidence`, or meaningful risk.

Customer-facing events may appear only when one of these is true:

- `stream_evidence_quality=transaction_enriched_mint`.
- `data_quality=live_rpc_evidence` and target is a real token mint.
- Risk index is meaningful enough to require monitoring/review.
- A red-flag condition exists, such as active mint authority, high holder concentration, or shared/concentrated wallet flow.

## 4. UI acceptance

The Security Radar page must use product language, not raw module language.

Allowed customer-facing actions:

- `Run Scan` — manual target analysis.
- `Refresh Feed` — refreshes live radar feed.
- `View Evidence` — opens the selected event evidence graph.
- `Dashboard` — returns to product dashboard.
- `Ecosystem` — opens ecosystem page.

Disallowed confusing labels:

- `Open Graph` as a separate action name.
- `Professional Graph` as the main graph action.
- `Trusted Token` as a guarantee.
- `Safe` as an investment recommendation.

Green lane copy must mean:

```text
No Critical Risk Found / Monitor
```

It must not mean guaranteed safety or investment advice.

Red lane copy must mean:

```text
Suspicious / Avoid
```

## 5. Evidence Graph acceptance

`View Evidence` must show a readable map for the selected event:

```text
Creator / Source
→ Buyer Distribution or Linked Wallet Cluster
→ Token / Pool
→ Risk + Evidence
```

The evidence graph must include:

- Mint authority.
- Freeze authority.
- Holder concentration if available.
- Action / recommendation.
- Evidence explanation.

The graph may be lightweight in the customer UI, but raw backend graph evidence can be enriched later.

## 6. Hidden legacy risk modules

Legacy scoring modules must not appear as customer-facing modules.

They may run only as SBX-1 internal signals:

- `tx_decoder`
- `token_scanner`
- `wallet_score`
- `risk_scanner`
- `sybil_graph`
- `project_radar`

They must write to internal verdict signals as:

```text
sbx1_hidden_signal_pack
sbx1_hidden_risk_adjustment
sbx1_hidden_customer_surface=false
```

They may adjust SBX-1 risk, but they must not create a visible module list on the customer page.

## 7. Legal/product-safety language

The product must not claim:

- Guaranteed safe token.
- Guaranteed scam detection.
- Investment advice.
- AI-determined final grade.

The product may claim:

- Live Solana WSS monitoring.
- On-chain evidence-based risk classification.
- Mint authority checks.
- Holder concentration checks.
- Wallet-flow and cluster hints when evidence exists.
- Explicit evidence gaps when evidence is insufficient.

## 8. Manual release test

Before calling the release final, run this checklist manually:

1. Open `/security-radar`.
2. Confirm top copy says green is not guaranteed safety.
3. Confirm `Run Scan`, `Refresh Feed`, and `View Evidence` labels are visible where expected.
4. Confirm low-confidence `18/100 partial evidence` spam is not visible.
5. Click `Refresh Feed`.
6. Run a known Solana mint through `Run Scan`.
7. Click `View Evidence` on a visible card.
8. Confirm the evidence graph renders.
9. Check Railway logs for SBX-1 startup.
10. Confirm no API key or provider secret is exposed in browser-visible source.

## 9. Release state

A build can be called `final candidate` when:

- Railway deploy is green.
- `/security-radar` loads.
- Manual scan works for at least one real target.
- Feed does not show raw low-confidence spam.
- Graph opens from a feed/manual verdict card.
- Shield API routes are deployed and protected by API key auth.
- Owner/admin checks can confirm stream counters and latest event timestamps.

If any item fails, the release is not final; it is a deploy-after-fix candidate.
