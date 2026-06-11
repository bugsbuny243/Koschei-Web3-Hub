# Koschei Public Shield

Free, open-source, community-owned Web3 risk prevention for Solana and Ethereum.

## Mission

Koschei Public Shield helps users and builders detect avoidable on-chain risks before losses happen. The project focuses on public-good security modules that are non-custodial, read-only, and transparent.

Koschei Public Shield is:

- **Non-profit in scope:** this repository contains only public safety tooling.
- **Open source:** released under the MIT License.
- **Community owned:** risk examples, heuristics, documentation, and integrations can be improved through issues and pull requests.
- **Non-custodial:** no private keys, no seed phrases, no custody.

## Included Modules

### MEV Shield

MEV Shield analyzes transaction risk signals such as slippage tolerance, trade size, and pool liquidity to estimate sandwich-attack exposure and prevented-loss value.

### Liquidity Radar

Liquidity Radar detects liquidity-drain and rug-pull patterns such as sudden reserve drops, early block exits, high removed liquidity, and high-severity alert conditions.

### Public Impact Dashboard

The dashboard aggregates public-good metrics such as:

- Total estimated USD loss prevented.
- Rug-pull / liquidity-drain alerts prevented.
- Active protected wallets.
- Largest anonymized prevention event in the last 24 hours.

Public production demo: `https://tradepigloball.co/impact`

## Excluded From This Repository

This repository intentionally excludes commercial and private operations code:

- Payment systems.
- Credit systems.
- Owner/admin panels.
- Private customer workflows.
- Proprietary commercial modules.

## Example Repository Structure

```text
koschei-public/
├── README.md
├── LICENSE
├── CONTRIBUTING.md
├── SECURITY.md
├── docs/
│   ├── architecture.md
│   ├── impact-methodology.md
│   └── qf-donor-guide.md
├── modules/
│   ├── mev-shield/
│   │   ├── README.md
│   │   └── heuristics.md
│   └── liquidity-radar/
│       ├── README.md
│       └── heuristics.md
├── examples/
│   ├── solana/
│   └── ethereum/
└── tests/
    ├── mev-shield.test.json
    └── liquidity-radar.test.json
```

## Public-Good Roadmap

- Publish MEV Shield heuristics and test fixtures.
- Publish Liquidity Radar heuristics and test fixtures.
- Add Solana and Ethereum integration examples.
- Add community-maintained risk examples.
- Publish transparent impact methodology.
- Maintain a donor-funded public dashboard.

## Contributing

You can contribute by:

- Reporting suspicious MEV, liquidity-drain, or rug-pull examples.
- Adding test fixtures.
- Improving documentation.
- Reviewing heuristics.
- Translating user safety guides.
- Building chain-specific adapters.

Please do not submit private keys, seed phrases, personally identifiable information, or exploit instructions that enable harm.

## License

MIT License. See `LICENSE`.
