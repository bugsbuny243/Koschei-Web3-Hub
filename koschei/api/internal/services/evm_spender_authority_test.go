package services

import (
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestBindEVMApprovalSpenderAuthorityKeepsEvidenceObservedOnly(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatal(err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"

	code := networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "contract_code_observed",
		ContractCodeHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		DelegationState:   networktarget.EVMDelegationStateObserved,
		DelegationTarget:  "0x3333333333333333333333333333333333333333",
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}
	proxy := networktarget.EVMProxyAuthorityResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		State:             networktarget.EVMProxyAuthorityObserved,
		Implementation:    "0x4444444444444444444444444444444444444444",
		Admin:             "0x5555555555555555555555555555555555555555",
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}
	authority, err := BuildEVMSpenderAuthoritySnapshot(code, proxy)
	if err != nil {
		t.Fatal(err)
	}
	if !authority.Trust.Observed || authority.Trust.Authorized || authority.Trust.Verified || authority.Trust.Finalized {
		t.Fatalf("authority was over-promoted: %+v", authority.Trust)
	}

	approval, err := BindEVMApprovalSpenderAuthority(EVMApprovalObservation{
		Network:    "ethereum-mainnet",
		Token:      "0x1111111111111111111111111111111111111111",
		Owner:      "0x9999999999999999999999999999999999999999",
		Spender:    resolution.Address,
		Mechanism:  EVMApprovalMechanismApprove,
		Amount:     "1000000",
		ObservedAt: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
	}, authority)
	if err != nil {
		t.Fatal(err)
	}
	if approval.SpenderDelegationTarget != "0x3333333333333333333333333333333333333333" || approval.ProxyImplementation != "0x4444444444444444444444444444444444444444" || approval.ProxyAdmin != "0x5555555555555555555555555555555555555555" {
		t.Fatalf("spender authority not bound: %+v", approval)
	}
	if !approval.Trust.Observed || approval.Trust.Authorized || approval.Trust.Verified || approval.Trust.Finalized {
		t.Fatalf("approval was over-promoted: %+v", approval.Trust)
	}
	assertReason := func(reason string) {
		t.Helper()
		for _, got := range approval.Reasons {
			if got == reason {
				return
			}
		}
		t.Fatalf("missing reason %s in %v", reason, approval.Reasons)
	}
	assertReason("READ_ONLY_EVM_APPROVAL_OBSERVATION")
	assertReason("READ_ONLY_SPENDER_AUTHORITY_OBSERVATION")
	assertReason("EIP7702_SPENDER_DELEGATION_OBSERVED")
	assertReason("SPENDER_UPGRADE_AUTHORITY_OBSERVED")
}

func TestBindEVMApprovalSpenderAuthorityRejectsSubjectMismatch(t *testing.T) {
	authority := EVMSpenderAuthoritySnapshot{
		Network: "ethereum-mainnet",
		Spender: "0x2222222222222222222222222222222222222222",
		Trust:   Web3TrustVector{Observed: true},
	}
	_, err := BindEVMApprovalSpenderAuthority(EVMApprovalObservation{
		Network:    "ethereum-mainnet",
		Token:      "0x1111111111111111111111111111111111111111",
		Owner:      "0x9999999999999999999999999999999999999999",
		Spender:    "0x7777777777777777777777777777777777777777",
		Mechanism:  EVMApprovalMechanismApprove,
		Amount:     "1",
		ObservedAt: time.Now().UTC(),
	}, authority)
	if err == nil {
		t.Fatal("expected spender identity mismatch to fail closed")
	}
}

func TestBuildEVMSpenderAuthoritySnapshotSurfacesConflictingERC1967Slots(t *testing.T) {
	resolution, err := networktarget.Resolve("base-mainnet", "0x2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatal(err)
	}
	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"

	code := networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x2105",
		ExpectedChainID:   "0x2105",
		ContractCodeState: "contract_code_observed",
		DelegationState:   networktarget.EVMDelegationStateNotObserved,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}
	proxy := networktarget.EVMProxyAuthorityResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x2105",
		ExpectedChainID:   "0x2105",
		State:             networktarget.EVMProxyAuthorityObserved,
		Implementation:    "0x4444444444444444444444444444444444444444",
		Beacon:            "0x6666666666666666666666666666666666666666",
		ConflictingSlots:  true,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}
	authority, err := BuildEVMSpenderAuthoritySnapshot(code, proxy)
	if err != nil {
		t.Fatal(err)
	}
	if !authority.ConflictingSlots {
		t.Fatal("expected conflicting ERC-1967 slots")
	}
	found := false
	for _, reason := range authority.Reasons {
		if reason == "ERC1967_CONFLICTING_AUTHORITY_SLOTS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing conflicting-slot reason: %v", authority.Reasons)
	}
}
