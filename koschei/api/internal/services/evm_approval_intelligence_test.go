package services

import (
	"strings"
	"testing"
	"time"
)

func TestBuildEVMApprovalIntelligenceKeepsPermitDeadlineSeparateFromAllowanceLifetime(t *testing.T) {
	observedAt := time.Unix(2_000, 0).UTC()
	result, err := BuildEVMApprovalIntelligence(EVMApprovalObservation{
		Network:            "ethereum-mainnet",
		Token:              "0x1111111111111111111111111111111111111111",
		Owner:              "0x2222222222222222222222222222222222222222",
		Spender:            "0x3333333333333333333333333333333333333333",
		Mechanism:          EVMApprovalMechanismPermit,
		Amount:             "1000",
		PermitNonce:        "7",
		PermitDeadlineUnix: 1_999,
		DomainSeparator:    "0x" + strings.Repeat("a", 64),
		ObservedAt:         observedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Persistence != EVMApprovalPersistencePersistent {
		t.Fatalf("persistence=%q", result.Persistence)
	}
	if !containsApprovalReason(result.Reasons, "PERMIT_SUBMISSION_WINDOW_CLOSED") {
		t.Fatalf("missing closed submission window reason: %v", result.Reasons)
	}
	if !containsApprovalReason(result.Reasons, "PERSISTENT_ALLOWANCE_SEMANTICS") {
		t.Fatalf("permit incorrectly modeled as expiring allowance: %v", result.Reasons)
	}
	if result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized || !result.Trust.Observed {
		t.Fatalf("permit observation was over-promoted: %+v", result.Trust)
	}
}

func TestBuildEVMApprovalIntelligenceModelsERC7674AsTransactionScoped(t *testing.T) {
	result, err := BuildEVMApprovalIntelligence(EVMApprovalObservation{
		Network:         "base-mainnet",
		Token:           "0x1111111111111111111111111111111111111111",
		Owner:           "0x2222222222222222222222222222222222222222",
		Spender:         "0x3333333333333333333333333333333333333333",
		Mechanism:       EVMApprovalMechanismTemporary,
		Amount:          "500",
		TransactionHash: "0x" + strings.Repeat("b", 64),
		ObservedAt:      time.Unix(3_000, 0).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Persistence != EVMApprovalPersistenceTransactionScope {
		t.Fatalf("persistence=%q", result.Persistence)
	}
	if !containsApprovalReason(result.Reasons, "ERC7674_TRANSACTION_SCOPED_APPROVAL_OBSERVED") {
		t.Fatalf("reasons=%v", result.Reasons)
	}
}

func TestBuildEVMApprovalIntelligenceFlagsUnlimitedAndUpgradeableSpenderAsObservedOnly(t *testing.T) {
	max := "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	result, err := BuildEVMApprovalIntelligence(EVMApprovalObservation{
		Network:             "arbitrum-mainnet",
		Token:               "0x1111111111111111111111111111111111111111",
		Owner:               "0x2222222222222222222222222222222222222222",
		Spender:             "0x3333333333333333333333333333333333333333",
		Mechanism:           EVMApprovalMechanismApprove,
		Amount:              max,
		ProxyImplementation: "0x4444444444444444444444444444444444444444",
		ProxyAdmin:          "0x5555555555555555555555555555555555555555",
		SpenderCodeHash:     strings.Repeat("c", 64),
		ObservedAt:          time.Unix(4_000, 0).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Unlimited || !containsApprovalReason(result.Reasons, "UNLIMITED_ALLOWANCE_OBSERVED") {
		t.Fatalf("unlimited allowance not surfaced: %+v", result)
	}
	if !containsApprovalReason(result.Reasons, "SPENDER_UPGRADE_AUTHORITY_OBSERVED") || !containsApprovalReason(result.Reasons, "SPENDER_CODE_OBSERVED") {
		t.Fatalf("spender authority evidence missing: %v", result.Reasons)
	}
	if result.Trust.Authorized || result.Trust.Verified || result.Trust.Finalized {
		t.Fatalf("upgrade observation was over-promoted: %+v", result.Trust)
	}
}

func TestBuildEVMApprovalIntelligenceRejectsMixedPermitAndTemporaryFields(t *testing.T) {
	_, err := BuildEVMApprovalIntelligence(EVMApprovalObservation{
		Network:         "optimism-mainnet",
		Token:           "0x1111111111111111111111111111111111111111",
		Owner:           "0x2222222222222222222222222222222222222222",
		Spender:         "0x3333333333333333333333333333333333333333",
		Mechanism:       EVMApprovalMechanismTemporary,
		Amount:          "1",
		PermitNonce:     "1",
		TransactionHash: "0x" + strings.Repeat("d", 64),
		ObservedAt:      time.Unix(5_000, 0).UTC(),
	})
	if err == nil {
		t.Fatal("expected mixed temporary/permit observation to fail closed")
	}
}

func containsApprovalReason(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
