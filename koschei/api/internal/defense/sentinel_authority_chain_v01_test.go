package defense

import "testing"

func TestVerifySentinelAuthorityChainAcceptsAttenuation(t *testing.T) {
	chain := []SentinelAuthorityGrant{
		{GrantID: "root", IssuerID: "owner", SubjectID: "sentinel", TenantID: "tenant-a", Capabilities: []string{"isolate_workload", "revoke_session"}, Targets: []string{"workload-1", "session-1"}, EvidenceIDs: []string{"ev-1", "ev-2"}},
		{GrantID: "child", IssuerID: "sentinel", SubjectID: "executor", TenantID: "tenant-a", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
	}
	if err := VerifySentinelAuthorityChain(chain); err != nil {
		t.Fatalf("expected attenuated chain to pass: %v", err)
	}
}

func TestVerifySentinelAuthorityChainRejectsCapabilityExpansion(t *testing.T) {
	chain := []SentinelAuthorityGrant{
		{GrantID: "root", IssuerID: "owner", SubjectID: "sentinel", TenantID: "tenant-a", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
		{GrantID: "child", IssuerID: "sentinel", SubjectID: "executor", TenantID: "tenant-a", Capabilities: []string{"isolate_workload", "disable_credential"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
	}
	if err := VerifySentinelAuthorityChain(chain); err == nil {
		t.Fatal("expected capability expansion to fail closed")
	}
}

func TestVerifySentinelAuthorityChainRejectsTenantBoundaryChange(t *testing.T) {
	chain := []SentinelAuthorityGrant{
		{GrantID: "root", IssuerID: "owner", SubjectID: "sentinel", TenantID: "tenant-a", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
		{GrantID: "child", IssuerID: "sentinel", SubjectID: "executor", TenantID: "tenant-b", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
	}
	if err := VerifySentinelAuthorityChain(chain); err == nil {
		t.Fatal("expected tenant boundary change to fail closed")
	}
}

func TestVerifySentinelAuthorityChainRejectsEvidenceExpansion(t *testing.T) {
	chain := []SentinelAuthorityGrant{
		{GrantID: "root", IssuerID: "owner", SubjectID: "sentinel", TenantID: "tenant-a", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1"}},
		{GrantID: "child", IssuerID: "sentinel", SubjectID: "executor", TenantID: "tenant-a", Capabilities: []string{"isolate_workload"}, Targets: []string{"workload-1"}, EvidenceIDs: []string{"ev-1", "ev-2"}},
	}
	if err := VerifySentinelAuthorityChain(chain); err == nil {
		t.Fatal("expected evidence scope expansion to fail closed")
	}
}
