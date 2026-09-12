package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
	"koschei/api/internal/web3"
)

const customerScanSchemaVersion = "koschei-customer-scan-v1"

type customerScanRequest struct {
	Target  string `json:"target"`
	Network string `json:"network,omitempty"`
}

type customerScanEnvelope struct {
	SchemaVersion string                      `json:"schema_version"`
	Result        services.CustomerScanResult `json:"result"`
}

type customerSolanaAccountInfo struct {
	Context struct {
		Slot uint64 `json:"slot"`
	} `json:"context"`
	Value *struct {
		Executable bool   `json:"executable"`
		Lamports   uint64 `json:"lamports"`
		Owner      string `json:"owner"`
		Space      uint64 `json:"space"`
	} `json:"value"`
}

func registerCustomerScanRoutes(mux *http.ServeMux, rpc ...*web3.SolanaRPC) {
	var solanaRPC *web3.SolanaRPC
	if len(rpc) > 0 {
		solanaRPC = rpc[0]
	}
	mux.HandleFunc("/api/scan", method(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		customerScanWithSolanaRPC(w, r, solanaRPC)
	}))
}

func decodeCustomerScanRequest(r io.Reader) (customerScanRequest, error) {
	var request customerScanRequest
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, errors.New("invalid_scan_request")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return request, errors.New("invalid_scan_request")
	}
	request.Target = strings.TrimSpace(request.Target)
	request.Network = strings.ToLower(strings.TrimSpace(request.Network))
	if request.Target == "" {
		return request, errors.New("scan_target_required")
	}
	if len(request.Target) > 256 || len(request.Network) > 64 {
		return request, errors.New("invalid_scan_request")
	}
	return request, nil
}

func customerScan(w http.ResponseWriter, r *http.Request) {
	customerScanWithSolanaRPC(w, r, nil)
}

func customerScanWithSolanaRPC(w http.ResponseWriter, r *http.Request, solanaRPC *web3.SolanaRPC) {
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	defer r.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		writeCustomerScanError(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	request, err := decodeCustomerScanRequest(r.Body)
	if err != nil {
		writeCustomerScanError(w, http.StatusBadRequest, err.Error())
		return
	}

	target, err := services.ClassifyCustomerScanTarget(request.Target, request.Network)
	if err != nil {
		writeCustomerScanError(w, http.StatusBadRequest, err.Error())
		return
	}
	if target.Route == services.CustomerScanRouteUnresolved || target.RequiresNetwork {
		result, buildErr := services.BuildCustomerScanResult(target, services.Web3TrustVector{}, nil)
		if buildErr != nil {
			writeCustomerScanError(w, http.StatusUnprocessableEntity, "scan_result_unavailable")
			return
		}
		writeCustomerScanResult(w, http.StatusUnprocessableEntity, result)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	observedAt := time.Now().UTC()

	if target.Route == services.CustomerScanRouteSolanaIntel {
		if request.Network != "solana-mainnet" {
			writeCustomerScanError(w, http.StatusUnprocessableEntity, "solana_mainnet_required")
			return
		}
		if solanaRPC == nil {
			target.Reasons = append(target.Reasons, "SOLANA_CUSTOMER_SCAN_NOT_CONNECTED")
			result, buildErr := services.BuildCustomerScanResult(target, services.Web3TrustVector{}, nil)
			if buildErr != nil {
				writeCustomerScanError(w, http.StatusUnprocessableEntity, "scan_result_unavailable")
				return
			}
			writeCustomerScanResult(w, http.StatusServiceUnavailable, result)
			return
		}
		var account customerSolanaAccountInfo
		params := []any{request.Target, map[string]any{"encoding": "base64", "commitment": "confirmed", "dataSlice": map[string]any{"offset": 0, "length": 0}}}
		if err := solanaRPC.Call(ctx, request.Network, "getAccountInfo", params, &account, 0); err != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "solana_account_probe_unavailable")
			return
		}
		observation := services.SolanaAccountObservation{
			Address: request.Target,
			Network: request.Network,
			Slot:    account.Context.Slot,
			Present: account.Value != nil,
		}
		if account.Value != nil {
			observation.Executable = account.Value.Executable
			observation.Lamports = account.Value.Lamports
			observation.Owner = strings.TrimSpace(account.Value.Owner)
			observation.Space = account.Value.Space
		}
		result, resultErr := services.CustomerScanResultFromSolanaObservation(target, observation)
		if resultErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "solana_evidence_projection_unavailable")
			return
		}
		_ = observedAt // observation slot is the primary evidence anchor; wall clock is not promoted to finality.
		writeCustomerScanResult(w, http.StatusOK, result)
		return
	}

	resolution, err := networktarget.Resolve(request.Network, request.Target)
	if err != nil {
		writeCustomerScanError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	switch target.Route {
	case services.CustomerScanRouteEVMProbe:
		endpoint := configuredEVMRPCEndpoint(resolution.Network.ID)
		if endpoint == "" {
			writeCustomerScanError(w, http.StatusServiceUnavailable, "evm_rpc_configuration_required")
			return
		}
		probe, probeErr := networktarget.ProbeEVM(ctx, nil, endpoint, resolution)
		if probeErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "evm_probe_unavailable")
			return
		}
		projection, projectionErr := services.AdaptEVMProbeEvidence(probe, observedAt)
		if projectionErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "evm_evidence_projection_unavailable")
			return
		}
		result, resultErr := services.CustomerScanResultFromNetworkProbe(target, projection)
		if resultErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "customer_scan_result_unavailable")
			return
		}

		proxyProbe, proxyErr := networktarget.ProbeEVMProxyAuthority(ctx, nil, endpoint, resolution)
		if proxyErr != nil {
			result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, "EVM_AUTHORITY_PROBE_UNAVAILABLE"))
			result.Trust.Reasons = append([]string(nil), result.Reasons...)
			writeCustomerScanResult(w, http.StatusOK, result)
			return
		}
		authority, authorityErr := services.BuildEVMSpenderAuthoritySnapshot(probe, proxyProbe)
		if authorityErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "evm_authority_projection_unavailable")
			return
		}
		result, resultErr = services.CustomerScanResultFromEVMAuthority(target, projection, authority)
		if resultErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "evm_authority_result_unavailable")
			return
		}
		writeCustomerScanResult(w, http.StatusOK, result)
	case services.CustomerScanRouteBitcoinProbe:
		endpoint := configuredBitcoinEsploraEndpoint()
		if endpoint == "" {
			writeCustomerScanError(w, http.StatusServiceUnavailable, "bitcoin_esplora_configuration_required")
			return
		}
		probe, probeErr := networktarget.ProbeBitcoin(ctx, nil, endpoint, resolution)
		if probeErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "bitcoin_probe_unavailable")
			return
		}
		projection, projectionErr := services.AdaptBitcoinProbeEvidence(probe, observedAt)
		if projectionErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "bitcoin_evidence_projection_unavailable")
			return
		}
		result, resultErr := services.CustomerScanResultFromNetworkProbe(target, projection)
		if resultErr != nil {
			writeCustomerScanError(w, http.StatusBadGateway, "customer_scan_result_unavailable")
			return
		}
		writeCustomerScanResult(w, http.StatusOK, result)
	default:
		result, buildErr := services.BuildCustomerScanResult(target, services.Web3TrustVector{Reasons: []string{"SCAN_ROUTE_NOT_CONNECTED"}}, nil)
		if buildErr != nil {
			writeCustomerScanError(w, http.StatusUnprocessableEntity, "scan_result_unavailable")
			return
		}
		writeCustomerScanResult(w, http.StatusNotImplemented, result)
	}
}

func writeCustomerScanResult(w http.ResponseWriter, status int, result services.CustomerScanResult) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(customerScanEnvelope{SchemaVersion: customerScanSchemaVersion, Result: result})
}

func writeCustomerScanError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version":     customerScanSchemaVersion,
		"error":              strings.TrimSpace(message),
		"analysis_performed": false,
		"evidence_status":    services.Web3TrustEvidenceUnverified,
	})
}
