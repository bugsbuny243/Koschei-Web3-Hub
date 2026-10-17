package securitycenter

import (
    "sort"

    "koschei/api/internal/runtimehealth"
)

// IntegrationReadinessSchemaVersion describes a read-only joining layer, not a
// proof that a repository capability is deployed, effective, or safe.
const IntegrationReadinessSchemaVersion = "koschei.security-center-integration-readiness.v1"

type RuntimeProbe struct {
    ID                string `json:"id"`
    State             string `json:"state"`
    Freshness         string `json:"freshness"`
    CoverageStatus    string `json:"coverage_status,omitempty"`
    CoverageAttention bool   `json:"coverage_attention_required,omitempty"`
    SuccessfulCycles  uint64 `json:"successful_cycles"`
}

type CapabilityReadiness struct {
    CapabilityID      string         `json:"capability_id"`
    RepositoryMode    string         `json:"repository_mode"`
    EvidenceAuthority string         `json:"evidence_authority"`
    RuntimeState      string         `json:"runtime_state"`
    ProbeIDs          []string       `json:"probe_ids,omitempty"`
    ObservedProbes    []RuntimeProbe `json:"observed_probes"`
    // Health and repository presence cannot substitute for accepted evidence.
    AcceptanceVerified bool `json:"acceptance_verified"`
}

type IntegrationReadiness struct {
    SchemaVersion    string                `json:"schema_version"`
    CapabilityCount  int                   `json:"capability_count"`
    RuntimeAvailable bool                  `json:"runtime_registry_available"`
    EvidenceRule     string                `json:"evidence_rule"`
    Limitations      []string              `json:"limitations"`
    Capabilities     []CapabilityReadiness `json:"capabilities"`
}

// These exact component IDs are registered by existing production call sites.
// A mapped component may still be only one dependency of the capability.
var capabilityRuntimeProbes = map[string][]string{
    "continuous-multi-network-block-ingest": {"worker.global-radar-head-ingest"},
    "global-radar-coverage-alert-lifecycle": {"worker.global-radar-coverage-alert-lifecycle"},
    "global-campaign-runtime": {"worker.global-campaign-materializer", "worker.global-radar-campaign-handoff"},
    "cross-process-job-wake": {"worker.cross-process-job-wake"},
    "network-and-validator-telemetry": {"worker.global-radar-background-telemetry"},
    "durable-evidence-memory": {"storage.application-postgres"},
}

// BuildIntegrationReadiness joins the current capability registry with an
// already sanitized runtime snapshot. It does not call collectors or alter state.
func BuildIntegrationReadiness(capabilities Snapshot, runtime *runtimehealth.Snapshot) IntegrationReadiness {
    out := IntegrationReadiness{
        SchemaVersion: IntegrationReadinessSchemaVersion,
        CapabilityCount: len(capabilities.Capabilities),
        RuntimeAvailable: runtime != nil,
        EvidenceRule: "runtime health is not chain safety, detector completeness, or G01-G08 acceptance proof",
        Limitations: []string{
            "Unmapped capabilities have no explicit runtime-to-capability link; they are not declared offline.",
            "A live dependency is only an operational observation; it does not verify end-to-end capability execution.",
            "No security verdict, grade, incident, alert, or stored evidence is modified by this projection.",
        },
        Capabilities: make([]CapabilityReadiness, 0, len(capabilities.Capabilities)),
    }
    observed := map[string]runtimehealth.Entry{}
    if runtime != nil {
        for _, entry := range runtime.Entries {
            observed[entry.ID] = entry
        }
    }
    for _, cap := range capabilities.Capabilities {
        row := CapabilityReadiness{
            CapabilityID: cap.ID, RepositoryMode: cap.Mode,
            EvidenceAuthority: cap.EvidenceAuthority,
            RuntimeState: "not_instrumented", ObservedProbes: []RuntimeProbe{},
        }
        if ids, ok := capabilityRuntimeProbes[cap.ID]; ok {
            row.ProbeIDs = append([]string{}, ids...)
            row.RuntimeState = "not_reported"
            if runtime == nil {
                row.RuntimeState = "runtime_registry_unavailable"
            } else {
                missing, attention, inactive, awaiting := false, false, false, false
                for _, id := range ids {
                    entry, found := observed[id]
                    if !found {
                        missing = true
                        continue
                    }
                    row.ObservedProbes = append(row.ObservedProbes, RuntimeProbe{
                        ID: entry.ID, State: string(entry.State), Freshness: entry.Freshness,
                        CoverageStatus: entry.CoverageStatus, CoverageAttention: entry.CoverageAttention,
                        SuccessfulCycles: entry.SuccessfulCycles,
                    })
                    switch {
                    case entry.CoverageAttention || entry.Freshness == "stale" ||
                        entry.State == runtimehealth.StateDegraded || entry.State == runtimehealth.StateUnavailable:
                        attention = true
                    case entry.State == runtimehealth.StateDisabled || entry.State == runtimehealth.StateStopped:
                        inactive = true
                    case entry.State != runtimehealth.StateLive || entry.SuccessfulCycles == 0:
                        awaiting = true
                    }
                }
                switch {
                case attention:
                    row.RuntimeState = "attention_required"
                case missing:
                    row.RuntimeState = "partially_reported"
                case inactive:
                    row.RuntimeState = "inactive"
                case awaiting:
                    row.RuntimeState = "awaiting_observation"
                case len(row.ObservedProbes) > 0:
                    row.RuntimeState = "observed_operational"
                }
            }
        }
        out.Capabilities = append(out.Capabilities, row)
    }
    sort.Slice(out.Capabilities, func(i, j int) bool {
        return out.Capabilities[i].CapabilityID < out.Capabilities[j].CapabilityID
    })
    return out
}
