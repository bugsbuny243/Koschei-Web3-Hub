# Koschei SBX-1 Collector Environment

SBX-1 is the owner-controlled Solana stream collector for Koschei Security Radar.

Secrets must stay in runtime environment variables. Do not hardcode provider keys, WebSocket URLs, RPC URLs, or tokens in repository files.

## Required runtime variables

```env
RADAR_STREAM_ENABLED=true
SOLANA_WSS_URL=wss://your-provider-solana-websocket-url
SOLANA_RPC_URL=https://your-provider-solana-https-rpc-url
```

Supported WSS fallback names, in priority order:

```env
SOLANA_WSS_URL=
ALCHEMY_SOLANA_WSS_URL=
HELIUS_SOLANA_WSS_URL=
QUICKNODE_SOLANA_WSS_URL=
```

## Optional tuning

```env
RADAR_STREAM_NETWORK=solana-mainnet
RADAR_EVENT_BUFFER_SIZE=5000
RADAR_STREAM_STORE_UNKNOWN=false
```

## What the collector does

- Opens an owner-controlled Solana WSS connection.
- Subscribes to confirmed Solana logs.
- Classifies Pump.fun, Raydium, and SPL Token hints from public logs.
- Persists raw stream evidence in `security_radar_stream_events`.
- Converts recognized events into customer-facing Security Radar verdicts.
- Sends stream evidence to the existing radar feed through backend persistence.

## What it must not do

- It must not embed API keys in source code.
- It must not deploy hidden scripts to third-party infrastructure.
- It must not claim sybil detection without on-chain evidence.
- It must not let AI change the final grade.
