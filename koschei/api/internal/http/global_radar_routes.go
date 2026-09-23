package http

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/radarevent"
	"koschei/api/internal/radargraph"
	"koschei/api/internal/radarpath"
	"koschei/api/internal/securityevidence"
)

const globalRadarSchemaVersion = "koschei.global-radar.v1"

type globalRadarTruthBoundary struct {
	LiveChainMetricsIncluded    bool   `json:"live_chain_metrics_included"`
	WalletGeolocationIncluded   bool   `json:"wallet_geolocation_included"`
	InfrastructureGeographyOnly bool   `json:"infrastructure_geography_only"`
	MissingEvidencePolicy       string `json:"missing_evidence_policy"`
	CapabilityIsNotLiveEvidence bool   `json:"capability_is_not_live_evidence"`
	HypothesisPromotionAllowed bool   `json:"hypothesis_promotion_allowed"`
}

type globalRadarNetworkState struct {
	Network          networktarget.Network `json:"network"`
	CollectorRuntime string                `json:"collector_runtime"`
	LiveAvailability string                `json:"live_availability"`
}

type globalRadarSummary struct {
	RegisteredNetworks    int      `json:"registered_networks"`
	NetworkFamilies       []string `json:"network_families"`
	ImplementationReady   int      `json:"implementation_ready"`
	ConfigurationRequired int      `json:"configuration_required"`
	LiveChecked           int      `json:"live_checked"`
}

type globalRadarSnapshot struct {
	SchemaVersion string                    `json:"schema_version"`
	EventSchema            string                    `json:"event_schema"`
	EntityGraphSchema      string                    `json:"entity_graph_schema"`
	AttackPathSchema       string                    `json:"attack_path_schema"`
	SecurityEvidenceSchema string                    `json:"security_evidence_schema"`
	Scope         string                    `json:"scope"`
	GeneratedAt   time.Time                 `json:"generated_at"`
	TruthBoundary globalRadarTruthBoundary  `json:"truth_boundary"`
	Summary       globalRadarSummary        `json:"summary"`
	Networks      []globalRadarNetworkState `json:"networks"`
}

func currentGlobalRadarSnapshot(now time.Time) globalRadarSnapshot {
	catalog := networktarget.Catalog()
	deployments := networkDeploymentCatalog()
	deploymentByNetwork := make(map[string]networkDeploymentState, len(deployments))
	for _, deployment := range deployments {
		deploymentByNetwork[deployment.NetworkID] = deployment
	}

	families := map[string]struct{}{}
	states := make([]globalRadarNetworkState, 0, len(catalog))
	implementationReady := 0
	configurationRequired := 0
	liveChecked := 0

	for _, network := range catalog {
		families[network.Family] = struct{}{}
		if network.CollectorStatus == "existing" || network.CollectorStatus == "probe_ready" {
			implementationReady++
		}

		deployment, ok := deploymentByNetwork[network.ID]
		if !ok {
			deployment = networkDeploymentState{
				NetworkID:        network.ID,
				CollectorRuntime: "unknown",
				LiveAvailability: "not_checked",
			}
		}
		if deployment.CollectorRuntime == "configuration_required" {
			configurationRequired++
		}
		if deployment.LiveAvailability == "checked" {
			liveChecked++
		}
		states = append(states, globalRadarNetworkState{
			Network:          network,
			CollectorRuntime: deployment.CollectorRuntime,
			LiveAvailability: deployment.LiveAvailability,
		})
	}

	familyList := make([]string, 0, len(families))
	for family := range families {
		familyList = append(familyList, family)
	}
	sort.Strings(familyList)

	return globalRadarSnapshot{
		SchemaVersion:          globalRadarSchemaVersion,
		EventSchema:            radarevent.SchemaVersionV1,
		EntityGraphSchema:      radargraph.SchemaVersionV1,
		AttackPathSchema:       radarpath.SchemaVersionV1,
		SecurityEvidenceSchema: securityevidence.SchemaVersionV1,
		Scope:         "multi-network-observation-control-plane",
		GeneratedAt:   now.UTC(),
		TruthBoundary: globalRadarTruthBoundary{
			LiveChainMetricsIncluded:    false,
			WalletGeolocationIncluded:   false,
			InfrastructureGeographyOnly: true,
			MissingEvidencePolicy:       "unknown",
			CapabilityIsNotLiveEvidence: true,
			HypothesisPromotionAllowed: false,
		},
		Summary: globalRadarSummary{
			RegisteredNetworks:    len(catalog),
			NetworkFamilies:       familyList,
			ImplementationReady:   implementationReady,
			ConfigurationRequired: configurationRequired,
			LiveChecked:           liveChecked,
		},
		Networks: states,
	}
}

func globalRadarSnapshotHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(currentGlobalRadarSnapshot(time.Now()))
}
