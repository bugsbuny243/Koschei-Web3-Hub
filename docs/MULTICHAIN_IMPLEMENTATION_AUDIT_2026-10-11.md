# Multi-chain implementation audit — 2026-10-11

Source: KOSCHEI-COK-AGLI-RADAR-v2-2026-10-11.md. This is a baseline audit, NOT an implementation-complete claim.

## Verified repository facts
- koschei/api/go.mod declares Go 1.26.9 (the uploaded plan requests 1.26.7; use the actual repository toolchain).
- internal/services/universal_investigation_registry.go declares EVM as UniversalAdapterProbe.
- internal/services/universal_investigation_dispatcher.go builds plans and routes but does not itself execute collectors.
- internal/services/customer_scan_result.go attaches InvestigationPlan as metadata in BuildCustomerScanResult.

## Ordered work and current status
| Step | Status | Evidence / next work |
|---|---|---|
| P1.1 | PARTIAL | Define actual shared evidence type and adapter contract after tracing verdict core |
| P1.2 | PARTIAL | Wire dispatcher to real collectors and fail closed |
| P1.3 | PARTIAL | Derive executable capabilities from registered conformance-tested adapters |
| P1.4 | PARTIAL | Recorded fixture harness and CI |
| P2.1 Tron | SKIPPED | Not yet implemented or tested |
| P2.2 EVM depth | PARTIAL | Existing read-only probe; grading must remain disabled |
| P2.3 TON | SKIPPED | No executable collector verified |
| P2.4 Cosmos/IBC | SKIPPED | No executable collector verified |
| P2.5 Stellar | SKIPPED | No executable collector verified |
| P3 target classes | SKIPPED | Not verified |
| P4 provider reality | SKIPPED | Provider usage not measured |
| P5 capability matrix | SKIPPED | Not generated |

## Safety and verification gates
No adapter may claim VERIFIED without chain identity, block/ledger height, provider host, raw-response hash and independent second-provider confirmation for critical findings. Missing evidence is UNKNOWN, not SAFE. EVM observations do not affect grades without explicit owner approval.

## Next engineering operation
Trace existing EvidenceRow / IntelligenceEvidence and EvaluateUnifiedRadarVerdict types and the shared RPC budget. Implement P1.1 in an isolated branch, run gofmt, go vet and go test with Go 1.26.9, then commit only on green tests. Continue steps sequentially.
