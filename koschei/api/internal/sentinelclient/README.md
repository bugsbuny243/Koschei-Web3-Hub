# Web3 -> Sentinel adapter

This package is the only customer-scan integration boundary from Koschei Web3 Hub into Koschei Sentinel.

The adapter is disabled unless `KOSCHEI_SENTINEL_OBSERVE_ENABLED=1` and a valid `KOSCHEI_SENTINEL_BASE_URL` is configured. Production enablement also requires the matching runtime API token.

Only signed ARVIS results with live evidence are projected. Sentinel is commentary-only, and a returned case ID or verdict signature mismatch is rejected. Sentinel output cannot alter ARVIS grade, signature, evidence or decision policy.
