# Koschei Web3 Hub

Koschei Web3 is a **Web3 Security Validation & Risk Intelligence Platform**. The repository is evidence-first: missing evidence remains `UNKNOWN`, chain-specific collection stays behind adapters, and customer-facing security decisions must remain deterministic, explainable, and attributable to evidence.

> This repository does not collapse Koschei Lang or Koschei Sentinel into Web3. Cross-project integration uses versioned Koschei Fabric contracts/adapters and starts in observe/shadow mode.

## Current state

Start here before making changes:

- [`PROJECT_STATE.md`](PROJECT_STATE.md) — current implementation checkpoint, blockers, acceptance status, and project boundaries.
- [`AGENTS.md`](AGENTS.md) — repository contract and authority boundaries.
- [`docs/PRODUCT_DIRECTION_2026-09-10.md`](docs/PRODUCT_DIRECTION_2026-09-10.md) — current product direction and all-network/all-address scope.
- [`docs/WEB4_WEB5_STANDARDS_INTAKE_2026-09-14.md`](docs/WEB4_WEB5_STANDARDS_INTAKE_2026-09-14.md) — current standards intake for agent interoperability, identity, and internet-native authorization/payment primitives.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contribution rules.
- [`SECURITY.md`](SECURITY.md) — private vulnerability reporting and secret-handling policy.

## Repository map

### `koschei/api`

Primary Go Web3 runtime and HTTP/API surface. The runtime owns Web3 evidence collection/normalization, deterministic analysis, customer decision contracts, entitlement enforcement, and Web3 product integration.

Key rules:

- Application persistence and commercial entitlement persistence are separate concerns.
- Missing evidence is not converted to a safe result.
- Stateful jobs/history/watchlists must fail closed when the required persistence path is unavailable.
- Chain-specific evidence belongs behind adapters; core decision semantics remain chain-independent.

### `oss/event-normalizer`

Standalone TypeScript normalization package for observation/event data. It has its own build, type-check, and test lifecycle and must not silently widen Web3 native decision schemas.

### `docs`

Architecture, acceptance, product-boundary, security, data-flow, deployment, and operator documentation. Documents describing a future capability are not production-completion evidence.

### `.github/workflows`

CI, security, release, smoke, and acceptance gates. A file or test definition is not proof of completion; use exact-head executed evidence.

## Local verification

### Go runtime

```bash
cd koschei/api
go test ./...
go vet ./...
```

Use the repository CI workflows for the authoritative vulnerability/static-security gates (`govulncheck`, `gosec`, secret scanning, release gates, and other acceptance jobs).

### Event normalizer

```bash
cd oss/event-normalizer
npm install --ignore-scripts --no-fund
npm run check
npm test
```

## Architecture boundaries

- **Koschei Web3 / ARVIS:** Web3 security validation, evidence, deterministic customer decisions, product integration, and case coordination.
- **Koschei Lang:** separate repository; owns language semantics, Matrix, compiler/runtime, capability/authority, policy/proof, and identity internals.
- **Koschei Sentinel:** separate repository; owns threat/model/evaluation/research interpretation and must not mint Web3 or Lang authority.
- **Koschei Fabric:** versioned integration contracts/adapters/events; it is a federation boundary, not a merged codebase.

## Web4 / Web5 policy

`Web4` and `Web5` are not treated here as magic version labels. Koschei adopts concrete, independently governed primitives only when they strengthen the product boundary. Current intake prioritizes MCP for agent-to-tool/data interoperability, A2A for agent-to-agent interoperability, W3C Verifiable Credentials for portable/verifiable claims, and separately gated HTTP-native payment/authorization primitives where a real Web3 customer flow requires them.

No external protocol may bypass ARVIS evidence rules, entitlement checks, signed-intent/effect controls, or Fabric ownership boundaries.

## Security

Never commit private keys, seed phrases, passwords, bearer tokens, API/provider secrets, bot tokens, webhook secrets, or customer-sensitive evidence. See [`SECURITY.md`](SECURITY.md) for reporting instructions.

## Status language

Use these terms precisely:

- **implemented** — code exists;
- **tested** — a test has executed against the relevant commit;
- **accepted** — required acceptance evidence exists for that exact candidate;
- **production-ready** — only after the defined release/operational gates pass.

Do not use one of these states as a substitute for another.
