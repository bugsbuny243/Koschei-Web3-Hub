package http

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

const networkProbeIntelligenceSchemaVersion = "koschei-network-probe-intelligence-v1"

type networkDeploymentState struct {
	NetworkID        string `json:"network_id"`
	CollectorRuntime string `json:"collector_runtime"`
	LiveAvailability string `json:"live_availability"`
}

type networkProbeIntelligenceEnvelope struct {
	SchemaVersion string                                      `json:"schema_version"`
	Probe         any                                         `json:"probe"`
	Intelligence  services.NetworkProbeIntelligenceProjection `json:"intelligence"`
}

func evmRPCEnvName(networkID string) (string, bool) {
	switch strings.TrimSpace(networkID) {
	case "ethereum-mainnet":
		return "ETHEREUM_RPC_URL", true
	case "base-mainnet":
		return "BASE_RPC_URL", true
	case "arbitrum-mainnet":
		return "ARBITRUM_RPC_URL", true
	case "optimism-mainnet":
		return "OPTIMISM_RPC_URL", true
	default:
		return "", false
	}
}

func configuredEVMRPCEndpoint(networkID string) string {
	name, ok := evmRPCEnvName(networkID)
	if !ok {
		return ""
	}
	return strings.TrimSpace(os.Getenv(name))
}

func configuredBitcoinEsploraEndpoint() string {
	return strings.TrimSpace(os.Getenv("BITCOIN_ESPLORA_URL"))
}

func networkDeploymentCatalog() []networkDeploymentState {
	states := make([]networkDeploymentState, 0, len(networktarget.Catalog()))
	for _, network := range networktarget.Catalog() {
		state := networkDeploymentState{NetworkID: network.ID, CollectorRuntime: network.CollectorStatus, LiveAvailability: "not_checked"}
		switch network.Family {
		case "evm":
			if configuredEVMRPCEndpoint(network.ID) == "" {
				state.CollectorRuntime = "configuration_required"
				state.LiveAvailability = "configuration_required"
			} else {
				state.CollectorRuntime = "rpc_configured"
			}
		case "utxo":
			if network.ID == "bitcoin-mainnet" {
				if configuredBitcoinEsploraEndpoint() == "" {
					state.CollectorRuntime = "configuration_required"
					state.LiveAvailability = "configuration_required"
				} else {
					state.CollectorRuntime = "esplora_configured"
					state.LiveAvailability = "not_checked"
				}
			}
		}
		states = append(states, state)
	}
	return states
}

func networkTargetProbe(w http.ResponseWriter, r *http.Request) {
	networkTargetProbeWithClient(w, r, nil)
}

func networkTargetProbeWithClient(w http.ResponseWriter, r *http.Request, client *http.Client) {
	networkTargetRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	defer r.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	withIntelligence := r.URL.Path == "/fabric/networks/probe/intelligence"
	isForm := mediaType == "application/x-www-form-urlencoded" && !withIntelligence

	reject := func(status int, message, availability string) {
		networkTargetRejected.Add(1)
		if isForm {
			renderNetworkCoverage(w, status, nil, message)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":              message,
			"analysis_performed": false,
			"evidence_status":    "unknown",
			"live_availability":  availability,
		})
	}

	var request networkTargetRequest
	if isForm {
		if r.ParseForm() != nil || len(r.PostForm) != 2 || len(r.PostForm["network"]) != 1 || len(r.PostForm["address"]) != 1 {
			reject(http.StatusBadRequest, "invalid_target_request", "not_checked")
			return
		}
		request.Network = r.PostForm.Get("network")
		request.Address = r.PostForm.Get("address")
	} else {
		if mediaType != "application/json" {
			reject(http.StatusUnsupportedMediaType, "json_required", "not_checked")
			return
		}
		var err error
		request, err = decodeNetworkTargetRequest(r.Body)
		if err != nil {
			reject(http.StatusBadRequest, "invalid_target_request", "not_checked")
			return
		}
	}

	resolution, err := networktarget.Resolve(request.Network, request.Address)
	if err != nil {
		reject(http.StatusUnprocessableEntity, err.Error(), "not_checked")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	writeIntelligence := func(probe any, projection services.NetworkProbeIntelligenceProjection) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(networkProbeIntelligenceEnvelope{
			SchemaVersion: networkProbeIntelligenceSchemaVersion,
			Probe:         probe,
			Intelligence:  projection,
		})
	}

	switch resolution.Network.Family {
	case "evm":
		endpoint := configuredEVMRPCEndpoint(resolution.Network.ID)
		if endpoint == "" {
			reject(http.StatusServiceUnavailable, "evm_rpc_configuration_required", "configuration_required")
			return
		}
		result, err := networktarget.ProbeEVM(ctx, client, endpoint, resolution)
		if err != nil {
			reject(http.StatusBadGateway, err.Error(), "unavailable")
			return
		}
		if withIntelligence {
			projection, projectionErr := services.AdaptEVMProbeEvidence(result, time.Now().UTC())
			if projectionErr != nil {
				reject(http.StatusBadGateway, "intelligence_projection_unavailable", "unavailable")
				return
			}
			writeIntelligence(result, projection)
			return
		}
		if isForm {
			renderNetworkProbeResult(w, http.StatusOK, &result, "")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	case "utxo":
		if resolution.Network.ID != "bitcoin-mainnet" {
			reject(http.StatusUnprocessableEntity, "live_probe_not_supported_for_network", "not_available")
			return
		}
		if isForm {
			reject(http.StatusUnprocessableEntity, "bitcoin_live_probe_requires_json", "not_available")
			return
		}
		endpoint := configuredBitcoinEsploraEndpoint()
		if endpoint == "" {
			reject(http.StatusServiceUnavailable, "bitcoin_esplora_configuration_required", "configuration_required")
			return
		}
		result, err := networktarget.ProbeBitcoin(ctx, client, endpoint, resolution)
		if err != nil {
			reject(http.StatusBadGateway, err.Error(), "unavailable")
			return
		}
		if withIntelligence {
			projection, projectionErr := services.AdaptBitcoinProbeEvidence(result, time.Now().UTC())
			if projectionErr != nil {
				reject(http.StatusBadGateway, "intelligence_projection_unavailable", "unavailable")
				return
			}
			writeIntelligence(result, projection)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	default:
		reject(http.StatusUnprocessableEntity, "live_probe_not_supported_for_network", "not_available")
	}
}
