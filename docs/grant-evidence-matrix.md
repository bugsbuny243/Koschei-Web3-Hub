# Grant Reviewer Evidence Matrix

This document maps the previous grant feedback to concrete repository outputs. It separates shipped code, live infrastructure and future grant milestones so roadmap work is not presented as complete.

| Reviewer requirement | Current evidence | Status |
| --- | --- | --- |
| Technical product | Go API, ARVIS worker runtime, event/verdict stores, authenticated analysis routes | Live |
| Solana-native infrastructure | Pump-style observation ingestion, Raydium analysis, Solana RPC enrichment and failover | Live |
| Open-source software | `oss/event-normalizer`, `oss/schemas/signed-verdict.schema.json` | Shipped |
| Developer tools | `sdk/typescript`, signed-verdict validator and `arvis-verdict` CLI | Shipped |
| Developer resources | API reference, developer quickstart, architecture, limitations and integration examples | Shipped |
| Reproducible quality | Go CI plus TypeScript compile, runtime test and package-validation workflow | Shipped |
| Broader ecosystem impact | Wallet warning and launchpad screening integration patterns | Shipped examples |
| Long-term infrastructure | Versioned schemas, rule versions, idempotent processing and evidence-withholding policy | Live design |
| External adoption | Builder pilots and case studies | Grant milestone |
| Package distribution | Versioned public npm releases | Grant milestone |

## Technical entry points

- Main architecture: `docs/ARCHITECTURE.md`
- Data flow: `docs/architecture/data-flow.md`
- API reference: `docs/api-reference.md`
- Developer quickstart: `docs/DEVELOPER_QUICKSTART.md`
- TypeScript SDK and CLI: `sdk/typescript`
- Event normalizer: `oss/event-normalizer`
- Signed verdict schema: `oss/schemas/signed-verdict.schema.json`
- Integration examples: `examples`
- Technical whitepaper: `docs/technical-whitepaper.md`

## Evidence policy

A reviewer should be able to distinguish three states:

- **Live** — used by the running production system.
- **Shipped** — present as usable code or documentation in this repository.
- **Grant milestone** — proposed future engineering work with acceptance criteria.

No planned endpoint, integration or package release should be described as live before it is implemented and tested.
