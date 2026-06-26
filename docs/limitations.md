# ARVIS Limitations

Koschei ARVIS provides risk intelligence, not a safety guarantee.

## Time boundary

A verdict reflects the evidence available when the analysis runs. Token, pool and wallet state can change later.

## Missing evidence

When verified evidence is unavailable, ARVIS should withhold the final verdict rather than inventing a grade.

## Monitor results

A monitor or low-risk result means no critical evidence was found in the current evidence window. It does not predict future behavior.

## Data sources

RPC limits, stale streams, provider outages and incomplete history can reduce evidence quality. Source freshness should be reviewed with the verdict.

## Identity boundary

Wallet and funding relations are technical observations. They are not confirmed real-world identity claims.

## AI boundary

AI may explain findings. Evidence and deterministic rules remain responsible for the final output.

## Integration responsibility

Partner applications should protect API keys, enforce timeouts, monitor quota use and show relevant evidence to users.
