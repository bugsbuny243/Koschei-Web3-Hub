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

const customerApprovalScanSchemaVersion = "koschei-approval-scan-v1"

type customerApprovalScanRequest struct {
	Network string `json:"network"`
	Token   string `json:"token"`
	Owner   string `json:"owner"`
	Spender string `json:"spender"`
}

type customerApprovalScanResult struct {
	Network          string                                `json:"network"`
	Token            string                                `json:"token"`
	Owner            string                                `json:"owner"`
	Spender          string                                `json:"spender"`
	Amount           string                                `json:"amount"`
	Unlimited        bool                                  `json:"unlimited"`
	EvidenceStatus   string                                `json:"evidence_status"`
	LiveAvailability string                                `json:"live_availability"`
	Trust            services.Web3TrustVector              `json:"trust"`
	Reasons          []string                              `json:"reasons"`
	SpenderAuthority *services.EVMSpenderAuthoritySnapshot `json:"spender_authority,omitempty"`
}

type customerApprovalScanEnvelope struct {
	SchemaVersion string                     `json:"schema_version"`
	Result        customerApprovalScanResult `json:"result"`
}

func decodeCustomerApprovalScanRequest(r io.Reader) (customerApprovalScanRequest, error) {
	var request customerApprovalScanRequest
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, errors.New("invalid_approval_scan_request")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return request, errors.New("invalid_approval_scan_request")
	}
	request.Network = strings.ToLower(strings.TrimSpace(request.Network))
	request.Token = strings.ToLower(strings.TrimSpace(request.Token))
	request.Owner = strings.ToLower(strings.TrimSpace(request.Owner))
	request.Spender = strings.ToLower(strings.TrimSpace(request.Spender))
	if request.Network == "" || request.Token == "" || request.Owner == "" || request.Spender == "" {
		return request, errors.New("approval_scan_context_required")
	}
	if len(request.Network) > 64 || len(request.Token) > 64 || len(request.Owner) > 64 || len(request.Spender) > 64 {
		return request, errors.New("invalid_approval_scan_request")
	}
	if _, ok := networktarget.ExpectedEVMChainID(request.Network); !ok {
		return request, errors.New("evm_allowance_unsupported_network")
	}
	return request, nil
}

func customerApprovalScan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		writeCustomerApprovalScanError(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	request, err := decodeCustomerApprovalScanRequest(r.Body)
	if err != nil {
		writeCustomerApprovalScanError(w, http.StatusBadRequest, err.Error())
		return
	}
	endpoint := configuredEVMRPCEndpoint(request.Network)
	if endpoint == "" {
		writeCustomerApprovalScanError(w, http.StatusServiceUnavailable, "evm_rpc_configuration_required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	allowance, err := networktarget.ProbeEVMAllowance(ctx, nil, endpoint, request.Network, request.Token, request.Owner, request.Spender)
	if err != nil {
		writeCustomerApprovalScanError(w, http.StatusBadGateway, "evm_allowance_probe_unavailable")
		return
	}

	reasons := []string{"CURRENT_ALLOWANCE_OBSERVED", "APPROVAL_PROVENANCE_NOT_INFERRED"}
	if allowance.Unlimited {
		reasons = append(reasons, "UNLIMITED_ALLOWANCE_OBSERVED")
	}
	trust := services.Web3TrustVector{Observed: true, Reasons: append([]string(nil), reasons...)}
	result := customerApprovalScanResult{
		Network:          allowance.Network,
		Token:            allowance.Token,
		Owner:            allowance.Owner,
		Spender:          allowance.Spender,
		Amount:           allowance.Amount,
		Unlimited:        allowance.Unlimited,
		EvidenceStatus:   allowance.EvidenceStatus,
		LiveAvailability: allowance.LiveAvailability,
		Trust:            trust,
		Reasons:          append([]string(nil), reasons...),
	}

	resolution, resolveErr := networktarget.Resolve(request.Network, request.Spender)
	if resolveErr != nil {
		result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, "SPENDER_AUTHORITY_UNAVAILABLE"))
		result.Trust.Reasons = append([]string(nil), result.Reasons...)
		writeCustomerApprovalScanResult(w, http.StatusOK, result)
		return
	}
	codeProbe, codeErr := networktarget.ProbeEVM(ctx, nil, endpoint, resolution)
	if codeErr != nil {
		result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, "SPENDER_AUTHORITY_UNAVAILABLE"))
		result.Trust.Reasons = append([]string(nil), result.Reasons...)
		writeCustomerApprovalScanResult(w, http.StatusOK, result)
		return
	}
	proxyProbe, proxyErr := networktarget.ProbeEVMProxyAuthority(ctx, nil, endpoint, resolution)
	if proxyErr != nil {
		result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, "SPENDER_AUTHORITY_UNAVAILABLE"))
		result.Trust.Reasons = append([]string(nil), result.Reasons...)
		writeCustomerApprovalScanResult(w, http.StatusOK, result)
		return
	}
	authority, authorityErr := services.BuildEVMSpenderAuthoritySnapshot(codeProbe, proxyProbe)
	if authorityErr != nil {
		result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, "SPENDER_AUTHORITY_UNAVAILABLE"))
		result.Trust.Reasons = append([]string(nil), result.Reasons...)
		writeCustomerApprovalScanResult(w, http.StatusOK, result)
		return
	}
	result.SpenderAuthority = &authority
	result.Reasons = services.NormalizeWeb3TrustReasons(append(result.Reasons, authority.Reasons...))
	result.Trust.Reasons = append([]string(nil), result.Reasons...)
	writeCustomerApprovalScanResult(w, http.StatusOK, result)
}

func writeCustomerApprovalScanResult(w http.ResponseWriter, status int, result customerApprovalScanResult) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(customerApprovalScanEnvelope{SchemaVersion: customerApprovalScanSchemaVersion, Result: result})
}

func writeCustomerApprovalScanError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version":     customerApprovalScanSchemaVersion,
		"error":              strings.TrimSpace(message),
		"analysis_performed": false,
		"evidence_status":    services.Web3TrustEvidenceUnverified,
	})
}
