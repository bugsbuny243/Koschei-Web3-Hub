# Koschei ARVIS Integration Examples

This directory contains small examples for Solana builders.

## Wallet warning

`wallet-warning/index.ts` calls the live Shield preflight route and converts the signed verdict into a simple wallet UI decision:

- block
- warn
- allow with monitoring
- allow

## Launchpad screening

`launchpad-screening/index.ts` queues a live API-key-protected token scan before a token is listed or promoted.

## Environment

```bash
export ARVIS_API_KEY="your-api-key"
```

The examples are intentionally small. Production integrations should add retries, timeout handling, logging, quota monitoring and secure secret storage.
