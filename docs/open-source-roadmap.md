# Open-Source Infrastructure Roadmap

Koschei ARVIS is a Solana-native security infrastructure project. The repository now contains reusable components that builders can run, inspect and integrate without depending on the customer dashboard.

## Shipped components

### 1. Solana event normalizer

Location: `oss/event-normalizer`

Status: shipped and tested.

The package converts Pump-style and Raydium-style observations into a deterministic versioned contract containing source, target, target type, network, timestamp and metadata. Missing timestamps or unclassified sources are rejected rather than silently invented.

### 2. Signed verdict schema

Location: `oss/schemas/signed-verdict.schema.json`

Status: shipped.

The schema defines the stable output contract used by developer integrations: grade, risk index, risk level, evidence, rule version and signed status.

### 3. TypeScript SDK

Location: `sdk/typescript`

Status: shipped and tested.

The dependency-free client supports the current production API routes and includes deterministic structural validation for signed verdicts.

### 4. Signed verdict CLI

Location: `sdk/typescript/bin/arvis-verdict.mjs`

Status: shipped and tested.

The command-line tool validates verdict JSON from a file or stdin and returns deterministic exit codes for valid, malformed and withheld outputs.

### 5. Integration examples

Location: `examples`

Status: shipped.

Current examples demonstrate:

- wallet preflight warnings
- launchpad token screening
- signed-verdict withholding when evidence is incomplete

### 6. Developer documentation

Locations:

- `docs/api-reference.md`
- `docs/DEVELOPER_QUICKSTART.md`
- `docs/architecture/data-flow.md`
- `docs/signed-verdict-schema.md`
- `docs/technical-whitepaper.md`

Status: shipped.

## Quality gates

The open-source TypeScript workflow runs for both packages and checks:

- strict TypeScript compilation
- runtime unit tests
- package build output
- package archive validation

The Go API has a separate test, vet and build workflow.

## Next 30 days

- publish versioned npm releases for the SDK and event normalizer
- add fixture-based Pump and Raydium normalization examples
- publish package checksums and release notes
- add webhook receiver examples after the webhook contract is finalized

## Next 60 days

- release a wallet integration starter
- release a launchpad screening starter
- add batch token-screening support
- publish deterministic benchmark fixtures and expected results

## Next 90 days

- run pilot integrations with Solana builders
- publish integration case studies
- expand wallet, token and sybil evidence coverage
- document false-positive review and rule-version governance

## Trust boundary

Open-source helpers do not declare an address malicious. They normalize observations, validate output structure and call evidence-backed APIs. When verified evidence is unavailable, integrations must withhold a final verdict rather than manufacture a score.
