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
)

type networkDeploymentState struct {
	NetworkID        string `json:"network_id"`
	CollectorRuntime string `json:"collector_runtime"`
	LiveAvailability string `json:"live_availability"`
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
			state.CollectorRuntime = network.CollectorStatus
			state.LiveAvailability = "collector_not_connected"
		}
		states = append(states, state)
	}
	return states
}

func networkTargetProbe(w http.ResponseWriter, r *http.Request) {
	networkTargetRequests.Add(1)
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	defer r.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	isForm := mediaType == "application/x-www-form-urlencoded"

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
			reject(http.StatusUnsupportedMediaType, "json_or_form_required", "not_checked")
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
	if resolution.Network.Family != "evm" {
		reject(http.StatusUnprocessableEntity, "live_probe_not_supported_for_network", "not_available")
		return
	}
	endpoint := configuredEVMRPCEndpoint(resolution.Network.ID)
	if endpoint == "" {
		reject(http.StatusServiceUnavailable, "evm_rpc_configuration_required", "configuration_required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := networktarget.ProbeEVM(ctx, nil, endpoint, resolution)
	if err != nil {
		reject(http.StatusBadGateway, err.Error(), "unavailable")
		return
	}
	if isForm {
		renderNetworkProbeResult(w, http.StatusOK, &result, "")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}
