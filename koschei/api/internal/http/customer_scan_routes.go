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

func registerCustomerScanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/scan", method(http.MethodPost, customerScan))
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
	return request, nil
}

func customerScan(w http.ResponseWriter, r *http.Request) {
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

	// Solana target classification is currently syntax-only here. The endpoint
	// must not manufacture live evidence until the existing Solana intelligence
	// collector is explicitly adapted to the CustomerScanResult contract.
	if target.Route == services.CustomerScanRouteSolanaIntel {
		target.Reasons = append(target.Reasons, "SOLANA_CUSTOMER_SCAN_NOT_CONNECTED")
		result, buildErr := services.BuildCustomerScanResult(target, services.Web3TrustVector{}, nil)
		if buildErr != nil {
			writeCustomerScanError(w, http.StatusUnprocessableEntity, "scan_result_unavailable")
			return
		}
		writeCustomerScanResult(w, http.StatusNotImplemented, result)
		return
	}

	resolution, err := networktarget.Resolve(request.Network, request.Target)
	if err != nil {
		writeCustomerScanError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	observedAt := time.Now().UTC()

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
		"schema_version": customerScanSchemaVersion,
		"error":          strings.TrimSpace(message),
		"analysis_performed": false,
		"evidence_status": services.Web3TrustEvidenceUnverified,
	})
}
