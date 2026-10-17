package securitycenter

import (
	"strings"
	"testing"

	"koschei/api/internal/runtimehealth"
)

func readinessTestSnapshot() Snapshot {
	return Snapshot{Capabilities: []Capability{
		{ID: "continuous-multi-network-block-ingest", Mode: "conditional", EvidenceAuthority: "observation_only"},
		{ID: "global-campaign-runtime", Mode: "conditional", EvidenceAuthority: "observation_only"},
		{ID: "operator-incident-workflow", Mode: "implemented", EvidenceAuthority: "operator_workflow_only"},
	}}
}

func readinessRow(t *testing.T, rows []CapabilityReadiness, id string) CapabilityReadiness {
	t.Helper()
	for _, row := range rows {
		if row.CapabilityID == id {
			return row
		}
	}
	t.Fatalf("missing capability %s", id)
	return CapabilityReadiness{}
}

func TestIntegrationReadinessNoRegistryFailsClosed(t *testing.T) {
	out := BuildIntegrationReadiness(readinessTestSnapshot(), nil)
	if out.RuntimeAvailable || out.CapabilityCount != 3 {
		t.Fatalf("missing registry was misreported: %#v", out)
	}
	head := readinessRow(t, out.Capabilities, "continuous-multi-network-block-ingest")
	if head.RuntimeState != "runtime_registry_unavailable" || head.AcceptanceVerified {
		t.Fatalf("missing registry promoted: %#v", head)
	}
	unmapped := readinessRow(t, out.Capabilities, "operator-incident-workflow")
	if unmapped.RuntimeState != "not_instrumented" || unmapped.AcceptanceVerified {
		t.Fatalf("unmapped capability promoted: %#v", unmapped)
	}
}

func TestIntegrationReadinessRegisteredDisabledAndMissing(t *testing.T) {
	r := runtimehealth.New()
	r.Register("worker.global-radar-head-ingest", "head_ingest", "ethereum-mainnet", false)
	out := BuildIntegrationReadiness(readinessTestSnapshot(), snapshotPtr(r))
	head := readinessRow(t, out.Capabilities, "continuous-multi-network-block-ingest")
	if head.RuntimeState != "inactive" {
		t.Fatalf("disabled worker: %#v", head)
	}
	campaign := readinessRow(t, out.Capabilities, "global-campaign-runtime")
	if campaign.RuntimeState != "partially_reported" && campaign.RuntimeState != "not_reported" {
		t.Fatalf("unobserved campaign worker: %#v", campaign)
	}
}

func TestIntegrationReadinessObservedButNotAccepted(t *testing.T) {
	r := runtimehealth.New()
	r.Register("worker.global-radar-head-ingest", "head_ingest", "ethereum-mainnet", true)
	r.Success("worker.global-radar-head-ingest", 0)
	out := BuildIntegrationReadiness(readinessTestSnapshot(), snapshotPtr(r))
	head := readinessRow(t, out.Capabilities, "continuous-multi-network-block-ingest")
	if head.RuntimeState != "observed_operational" || head.AcceptanceVerified || len(head.ObservedProbes) != 1 || head.ObservedProbes[0].SuccessfulCycles != 1 {
		t.Fatalf("operational probe must not verify security: %#v", head)
	}
}

func TestIntegrationReadinessCoverageBlindSpotOverridesLive(t *testing.T) {
	r := runtimehealth.New()
	r.Register("worker.global-radar-head-ingest", "head_ingest", "ethereum-mainnet", true)
	r.Success("worker.global-radar-head-ingest", 3)
	oldCursor, behindHead := uint64(110), uint64(100)
	r.RecordIngestProgress("worker.global-radar-head-ingest", &behindHead, &oldCursor, true, false)
	out := BuildIntegrationReadiness(readinessTestSnapshot(), snapshotPtr(r))
	head := readinessRow(t, out.Capabilities, "continuous-multi-network-block-ingest")
	if head.RuntimeState != "attention_required" || !head.ObservedProbes[0].CoverageAttention {
		t.Fatalf("blind spot was hidden by live state: %#v", head)
	}
}

func TestIntegrationReadinessPartialDependencyAndNoSecrets(t *testing.T) {
	r := runtimehealth.New()
	r.Register("worker.global-campaign-materializer", "worker", "", true)
	r.Success("worker.global-campaign-materializer", 1)
	r.Register("worker.global-radar-campaign-handoff", "worker", "", true)
	r.Failure("worker.global-radar-campaign-handoff", errFixture("secret DSN and provider endpoint"))
	out := BuildIntegrationReadiness(readinessTestSnapshot(), snapshotPtr(r))
	campaign := readinessRow(t, out.Capabilities, "global-campaign-runtime")
	if campaign.RuntimeState != "attention_required" || campaign.AcceptanceVerified {
		t.Fatalf("partial degraded dependency was hidden: %#v", campaign)
	}
	for _, item := range campaign.ObservedProbes {
		if strings.Contains(item.ID, "secret") || strings.Contains(item.State, "secret") {
			t.Fatalf("secret escaped through readiness: %#v", item)
		}
	}
}

type errFixture string

func (e errFixture) Error() string { return string(e) }

func snapshotPtr(r *runtimehealth.Registry) *runtimehealth.Snapshot {
	s := r.PublicSnapshot()
	return &s
}
