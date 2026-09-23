# Global Radar Live Probe Event Projection v1

Date: 2026-09-23

## Endpoint

`POST /fabric/radar/probe-event`

Request:

```json
{
  "network": "ethereum-mainnet",
  "address": "0x..."
}
```

The first runtime slice supports the existing live address probes for:

- registered EVM networks;
- Bitcoin mainnet.

## Output

The endpoint returns:

- the normalized live probe result;
- its SHA-256 binding;
- a sealed `koschei.global-radar-event.v1`.

The source binding is explicitly named:

`normalized_probe_json_sha256`

This does not claim that the digest is the provider's raw wire response. It binds the deterministic normalized probe object that the radar event was built from.

## Trust boundary

- the selected network is resolved before the live probe;
- EVM probes verify the RPC chain ID before accepting code evidence;
- Bitcoin verifies the mainnet genesis hash before accepting address activity;
- missing provider configuration fails closed;
- unsupported network families return `not_available`;
- the event adapter does not create a risk score or ARVIS verdict.

This is the first HTTP path where live non-Solana probe evidence is emitted directly into the Global Radar event contract.
