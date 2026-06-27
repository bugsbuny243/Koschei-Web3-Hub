# Persistent Watchlist and Change Alerts

Koschei Watchlist stores customer-owned Solana token targets and compares each new scan with the previous verified snapshot.

## Technology

- Go API
- Neon PostgreSQL persistence
- plain HTML/CSS/browser JavaScript at `/watchlist`
- no frontend framework
- no private key or seed phrase collection

## Routes

All routes require a Koschei customer session and an active entitlement.

```text
GET    /api/watchlist
POST   /api/watchlist
PATCH  /api/watchlist/{id}
DELETE /api/watchlist/{id}
POST   /api/watchlist/{id}/refresh
POST   /api/watchlist/refresh?limit=5
GET    /api/watchlist/alerts
POST   /api/watchlist/alerts
```

## Add a target

```json
{
  "target": "SOLANA_TOKEN_MINT",
  "target_type": "token",
  "network": "solana-mainnet",
  "label": "Launch candidate",
  "alert_threshold": 50
}
```

`alert_threshold` is a minimum security score. An alert is created when a previously healthy score crosses below this floor.

## Current alert rules

- security score drops by at least 15 points
- security score crosses below the configured floor
- mint authority changes
- freeze authority changes
- an authority that was previously disabled becomes active
- largest-holder concentration increases by at least 10 percentage points
- raw token supply changes

The first successful scan creates the baseline and does not create an alert.

## Refresh behavior

The first release refreshes targets through explicit customer actions. This is deliberate because Railway currently runs the Go API process and RPC capacity must be protected. Batch refresh is limited to ten targets per call and refreshes the oldest targets first.

Each successful scan schedules the next recommended check one hour later. A later scheduler can consume `next_check_at` without changing the customer API or database contract.

## Storage

`watchlist_targets` stores:

- customer ownership
- target and label
- active or paused status
- security floor
- latest verified snapshot
- last and next check timestamps

`watchlist_alerts` stores immutable before/after values, severity, evidence and read state. Deleting a target deletes its alert history through the foreign-key relationship.

## Compatibility

The feature is additive. Existing scanners, Radar, Transaction Firewall, authentication, entitlement and payment flows are unchanged.
