package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestCustomerScanResultFromEVMAuthorityExposesObservedAuthorityWithoutPromotion(t *testing.T) {
	const address = "0x1111111111111111111111111111111111111111"
	const admin = "0x2222222222222222222222222222222222222222"

	target, err := ClassifyCustomerScanTarget(address, "ethereum-mainnet")
	if err != nil {
		t.Fatalf("classify target: %v", err)
	}
	resolution, err := networktarget.Resolve("ethereum-mainnet", address)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = IntelligenceEvidenceObserved
	resolution.LiveAvailability = "checked"

	code := networktarget.EVMProbeResult{
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "contract_code_observed",
		ContractCodeHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		DelegationState:   networktarget.EVMDelegationStateNotObserved,
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}
	projection, err := AdaptEVMProbeEvidence(code, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("adapt code evidence: %v", err)
	}
	proxy := networktarget.EVMProxyAuthorityResult{
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		State:             networktarget.EVMProxyAuthorityObserved,
		Admin:             admin,
		AnalysisPerformed: true,
		EvidenceStatus:    IntelligenceEvidenceObserved,
		LiveAvailability:  "checked",
	}
	authority, err := BuildEVMSpenderAuthoritySnapshot(code, proxy)
	if err != nil {
		t.Fatalf("build authority: %v", err)
	}
	result, err := CustomerScanResultFromEVMAuthority(target, projection, authority)
	if err != nil {
		t.Fatalf("build customer result: %v", err)
	}
	if result.EVMAuthority == nil || result.EVMAuthority.ProxyAdmin != admin {
		t.Fatalf("expected proxy admin in customer authority result: %#v", result.EVMAuthority)
	}
	if !result.Trust.Observed {
		t.Fatal("expected observed trust")
	}
	if result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized {
		t.Fatalf("authority observation must not promote trust: %#v", result.Trust)
	}
}

func TestCustomerScanResultFromEVMAuthorityRejectsSubjectMismatch(t *testing.T) {
	const address = "0x1111111111111111111111111111111111111111"
	target, err := ClassifyCustomerScanTarget(address, "ethereum-mainnet")
	if err != nil {
		t.Fatalf("classify target: %v", err)
	}
	projection := NetworkProbeIntelligenceProjection{
		Subject: ClassifyIntelligenceSubject(address, "ethereum-mainnet"),
	}
	projection.Evidence = buildNetworkProbeEvidence(projection.Subject, "evm_rpc_probe", "eth_getCode", "contract_code_observed", time.Now().UTC(), map[string]any{
		"delegation_state": networktarget.EVMDelegationStateNotObserved,
	})
	authority := EVMSpenderAuthoritySnapshot{
		Network: "ethereum-mainnet",
		Spender: "0x3333333333333333333333333333333333333333",
		Trust:   Web3TrustVector{Observed: true},
	}
	if _, err := CustomerScanResultFromEVMAuthority(target, projection, authority); err == nil {
		t.Fatal("expected subject mismatch to fail closed")
	}
}
