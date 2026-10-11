# Multi-chain radar implementation gate — 2026-10-11

Source: KOSCHEI-COK-AGLI-RADAR-v2-2026-10-11.md. This document is an implementation checklist, not a claim of deployed capabilities.

## Verified baseline
- `koschei/api/go.mod` declares Go 1.26.9; do not downgrade to 1.26.7 from the source plan.
- `internal/services/universal_investigation_registry.go` contains textual adapter status declarations. These are not proof of executable collection.

## Ordered work (none marked complete)
- [ ] P1.1 shared adapter contract and evidence rows compatible with unified verdict core
- [ ] P1.2 execute registry dispatcher on the production scan path
- [ ] P1.3 derive executable capability status from registered conformant collectors
- [ ] P1.4 fixture conformance, Solana regression tests and CI
- [ ] P2.1 Tron permissions and two-provider confirmation
- [ ] P2.2 EVM proxy/approval/7702 evidence, grading disabled pending owner written approval
- [ ] P2.3 TON jetton identity and complete async trace evidence
- [ ] P2.4 Cosmos/IBC authz, feegrant and channel health
- [ ] P2.5 Stellar RPC/Hubble flags and trustlines
- [ ] P3.1 wallet/account authority
- [ ] P3.2 contract/program control
- [ ] P3.3 frontend/DNS/signature requests
- [ ] P3.4 bridge consistency and governance
- [ ] P4 provider configuration, shared budgets, failover and measured cost notes
- [ ] P5 generated capability matrix and proof-based status report

## Non-negotiable evidence policy
Missing != empty != safe. Provider outage != completed scan. Every finding must include chain identity, height/ledger/slot, provider and raw response hash. Critical VERIFIED findings require independent provider confirmation. Never represent a catalog entry as live without a conformant collector and fixture. No independent adapter grading. Do not introduce separate HTTP clients or bypass shared RPC budgets.

## Execution gate
Before claiming any step DONE: inspect its integration path, implement, run gofmt, go vet and go test with repository-declared Go version, and record commit and file:line proof. No such verification has been completed by this planning commit.
