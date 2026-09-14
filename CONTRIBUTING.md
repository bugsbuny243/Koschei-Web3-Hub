# Contributing to Koschei Web3 Hub

Koschei Web3 is a Web3 Security Validation & Risk Intelligence Platform. Contributions should preserve the evidence-first and fail-closed product contract described in `AGENTS.md` and `PROJECT_STATE.md`.

## Before changing code

Read:

- `README.md` for the repository map and local verification entrypoints;
- `PROJECT_STATE.md` for current blockers and acceptance truth;
- `AGENTS.md` for Web3 / Lang / Sentinel / Fabric authority boundaries;
- `SECURITY.md` before reporting or changing security-sensitive behavior.

Do not treat a design document, route registration, mock, fixture, test definition, or UI surface as production-completion evidence.

## Reporting bugs

Include:

- a clear description of the issue;
- steps to reproduce it;
- expected and actual behavior;
- affected commit/revision when known;
- browser/device/OS details when relevant;
- only public wallet, token, contract, transaction, or network identifiers needed to reproduce a Web3 issue.

Never include private keys, seed phrases, session/bearer tokens, passwords, provider/API secrets, bot/webhook secrets, database credentials, or customer-sensitive evidence.

Security vulnerabilities belong in the private reporting process described in `SECURITY.md`, not in a public issue with exploit details.

## Feature proposals

Describe:

- the customer/security problem;
- the proposed evidence source and trust model;
- supported networks/adapters;
- failure behavior when evidence is missing or stale;
- customer/operator surface and telemetry;
- authorization/entitlement impact;
- acceptance criteria and rollback/compatibility plan.

New chain support must be adapter-based. New agent/Web4/Web5 protocols must follow the protocol admission rules in `docs/PRODUCT_DIRECTION_2026-09-10.md` and the current standards intake in `docs/WEB4_WEB5_STANDARDS_INTAKE_2026-09-14.md`.

## Pull requests

Keep changes focused and easy to review. A PR should state:

- what changed and why;
- which trust/product boundary it affects;
- tests/checks executed against the exact head;
- remaining blockers or unavailable external acceptance;
- screenshots for visible UI changes;
- migration/deployment notes when state or configuration changes.

Do not bypass or weaken fail-closed behavior to make a test or demo pass.

## Core invariants

- Missing evidence remains `UNKNOWN`; it is never converted into `SAFE`.
- Chain adapters transport/normalize evidence; they do not own final decision semantics.
- ARVIS customer decisions remain deterministic, evidence-backed, and explainable.
- Commercial entitlement and application persistence remain explicit concerns.
- Lang and Sentinel remain separate repositories with their own authority boundaries.
- Fabric integrations are additive, versioned, observable, and initially reversible/observe-first.
- External MCP/A2A/credential/payment adapters cannot mint final Web3 authority or bypass native signing/effect gates.

## Verification

For the Go runtime:

```bash
cd koschei/api
go test ./...
go vet ./...
```

For the event normalizer:

```bash
cd oss/event-normalizer
npm install --ignore-scripts --no-fund
npm run check
npm test
```

Repository CI is authoritative for the configured secret scan, vulnerability scan, static security checks, release gates, smoke checks, and acceptance workflows. Exact-head evidence matters: an older green run does not validate newer commits.