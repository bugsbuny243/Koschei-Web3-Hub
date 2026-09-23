# Koschei Global Radar Node Health v1

Date: 2026-09-23

## Purpose

The Global Radar network-event path now supports live EVM execution-node telemetry in addition to Sui and Aptos network identity evidence.

`POST /fabric/radar/network-event`

Example:

```json
{"network":"ethereum-mainnet"}
```

The configured RPC endpoint is checked against the requested chain ID before node telemetry is accepted.

## EVM execution-node evidence

The event may preserve:

- observed chain ID and expected chain ID;
- client version;
- peer count for the configured endpoint;
- observed head block;
- sync state;
- explicit `single_rpc_endpoint_only` scope.

These values describe the configured RPC node only. They are **not** presented as:

- global node count;
- global validator count;
- decentralization score;
- censorship verdict;
- network-wide client distribution.

The normalized telemetry probe is SHA-256 bound before being projected into `koschei.global-radar-event.v1`.

## Beacon / consensus adapter

The radar-event layer also contains an Ethereum Beacon telemetry adapter capable of preserving:

- connected peers;
- head slot and sync distance;
- optimistic / execution-layer-offline state;
- previous/current justified checkpoints;
- finalized checkpoint evidence.

That adapter remains separate from the generic execution-node HTTP route until an explicit consensus endpoint/configuration boundary is promoted.

## Evidence boundary

Node-health observations remain OBSERVED unless their underlying evidence contract explicitly supports VERIFIED state.

A reachable or synced node does not mean the network or an asset is safe. Node telemetry is one evidence plane that may later participate in provider-divergence and network-health correlation.
