package handlers

// Handler routing status audit (2026-09-14).
//
// Category A — superseded handlers; not routed, kept for reference:
// security_radar_detail.go: SUPERSEDED BY security_radar_detail_v3.go. Not routed. Kept for reference.
// public_case_operational.go: SUPERSEDED BY public_case_operational_v2.go. Not routed. Kept for reference.
// public_case_page.go: SUPERSEDED BY PublicCaseOperationalPageV2. Not routed. Kept for reference.
// transaction_guard_v2.go: SUPERSEDED BY transaction_guard_v3.go. Not routed. Kept for reference.
// public_dossier_cases_v2.go: SUPERSEDED BY public_dossier_registry_drive.go for routed discovery. Not routed. Kept for reference; its shared loader/types remain in use.
//
// Category B — shared live definitions; intentionally untouched:
// credits.go
// entitlement.go
// member_summary.go
// dossier_export.go
// public_case_summary.go
// customer_investigation_history.go
// transaction_guard_v2_evidence_first.go
//
// Category C — inspected, not routed because they are not production-safe owner ARVIS surfaces:
// rug_radar.go: legacy heuristic score can emit SAFE; conflicts with evidence-first ARVIS semantics.
// liquidity_radar.go: scores client-supplied before/after percentages and can emit webhook side effects; not a canonical collector.
// dao_guardian.go: owner summary is placeholder/incomplete rather than live governance evidence.
// impact_metrics.go: coupled to the legacy MEV Shield/protected-transaction pivot.
// metadata.go: customer subscription/user auth semantics; not a clean owner-only ARVIS handler.
//
// Category D — OUT OF SCOPE. Not routed:
// web3.go: OUT OF SCOPE. Not routed.
// jobs.go: OUT OF SCOPE. Not routed.
// package_status.go: OUT OF SCOPE. Not routed.
// local_auth.go: OUT OF SCOPE. Not routed.
// owner_payment_health.go: OUT OF SCOPE. Not routed.
// mev_shield.go: OUT OF SCOPE. Not routed. Transaction-sending behavior conflicts with the no-custody boundary.
// plans.go: OUT OF SCOPE. Not routed.
