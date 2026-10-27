package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"koschei/api/internal/jobs"
	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

type asyncWeb3Request struct {
	Mint      string `json:"mint"`
	Address   string `json:"address"`
	Signature string `json:"signature"`
	Network   string `json:"network"`
	MaxDepth  int    `json:"max_depth,omitempty"`
}

func (h *Handler) CreateWeb3Job(w http.ResponseWriter, r *http.Request) {
	if h.JobStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "job service unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	jobType := web3JobTypeFromPath(r.URL.Path)
	if jobType == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown job type"})
		return
	}
	var req asyncWeb3Request
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Network == "" {
		req.Network = defaultNetworkForJob(jobType)
	}
	target := strings.TrimSpace(req.Mint)
	if target == "" {
		target = strings.TrimSpace(req.Address)
	}
	if target == "" {
		target = strings.TrimSpace(req.Signature)
	}
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target required"})
		return
	}
	// Only create a customer-paid job when a real adapter accepts this network
	// and address. EVM/UTXO use native evidence-only workers; the existing
	// Solana token/wallet investigation retains its own canonical classifier.
	if jobType == CanonicalInvestigationJobType && req.Network != "solana-mainnet" {
		network, registered := networktarget.LookupNetwork(req.Network)
		if !registered || (network.Family != "evm" && req.Network != "bitcoin-mainnet") {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "canonical_job_network_collector_not_connected", "network": req.Network,
			})
			return
		}
		classified, classifyErr := services.ClassifyCustomerScanTarget(target, req.Network)
		if classifyErr != nil || classified.RequiresNetwork || classified.Route == services.CustomerScanRouteUnresolved {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "canonical_job_target_network_mismatch", "network": req.Network,
			})
			return
		}
		// Transactions are validated by the chain-specific canonical hash
		// classifier; address probes still require networktarget.Resolve.
		if classified.Route == services.CustomerScanRouteTxLookup {
			if classified.Kind != services.CustomerScanTargetTxHash ||
				(network.Family == "evm" && !strings.HasPrefix(target, "0x")) ||
				(req.Network == "bitcoin-mainnet" && strings.HasPrefix(target, "0x")) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
					"error": "canonical_job_target_network_mismatch", "network": req.Network,
				})
				return
			}
		} else if _, err := networktarget.Resolve(req.Network, target); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "canonical_job_target_network_mismatch", "network": req.Network,
			})
			return
		}
		// Do not meter or queue an investigation that cannot obtain any
		// native evidence because its provider is not configured.
		if network.Family == "evm" && canonicalNativeRPCEnv(req.Network) == "" ||
			req.Network == "bitcoin-mainnet" && strings.TrimSpace(os.Getenv("BITCOIN_ESPLORA_URL")) == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "canonical_native_collector_configuration_required", "network": req.Network,
			})
			return
		}
	}
	requestPayload := any(req)
	if jobType == CanonicalInvestigationJobType && req.Network != "solana-mainnet" {
		requestPayload = canonicalInvestigationJobPayload{
			Address: target, Network: req.Network, Mode: "customer_canonical_job",
			RootTarget: target, Source: "customer_radar_job", Depth: 0, MaxDepth: 0,
		}
	} else if jobType == CanonicalInvestigationJobType {
		classification := classifyRadarTarget(r.Context(), target)
		resolvedTarget := target
		if classification.Type == radarTargetTokenAccount && strings.TrimSpace(classification.TokenOwnerWallet) != "" {
			resolvedTarget = strings.TrimSpace(classification.TokenOwnerWallet)
		}
		switch classification.Type {
		case radarTargetTokenMint, radarTargetWallet, radarTargetTokenAccount:
		default:
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": "unsupported_canonical_job_target", "target_classification": classification,
			})
			return
		}
		maxDepth := req.MaxDepth
		if maxDepth <= 0 {
			maxDepth = canonicalWorkerEnvInt("ACTOR_RECURSIVE_MAX_DEPTH", 1, 1, 3)
		}
		// Match owner route's existing 3-hop recursive upper bound.
		if maxDepth > 3 {
			maxDepth = 3
		}
		payload := canonicalInvestigationJobPayload{
			Address: resolvedTarget, Network: req.Network, Mode: "customer_canonical_job",
			RootTarget: resolvedTarget, Source: "customer_radar_job", Depth: 0, MaxDepth: maxDepth,
		}
		if classification.Type == radarTargetTokenMint {
			payload.Mint, payload.Address = resolvedTarget, ""
		}
		target, requestPayload = resolvedTarget, payload
	}
	// Invalid native targets must be refused before any customer usage debit.
	if _, err := h.requirePremiumOutput(claims.Sub); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	input := jobs.CreateInput{
		UserID: claims.Sub, Email: claims.Email, Type: jobType,
		Network: req.Network, Target: target, Request: requestPayload,
	}
	var job jobs.Job
	created := true
	var err error
	if jobType == CanonicalInvestigationJobType {
		// A repeated request for the same account, target and network joins the
		// queued or running investigation instead of paying for a second one.
		job, created, err = h.JobStore.CreateUniqueInFlight(r.Context(), input, "customer|"+claims.Sub)
	} else {
		job, err = h.JobStore.Create(r.Context(), input)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "job create failed"})
		return
	}
	pollURL := "/api/jobs/" + job.ID
	if jobType == CanonicalInvestigationJobType {
		pollURL = "/api/v1/radar/jobs/" + job.ID
	}
	status := http.StatusCreated
	if !created {
		w.Header().Set("X-Koschei-Quota-Consume", "false")
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"job_id": job.ID, "status": job.Status, "poll_url": pollURL,
		"package_required": true, "canonical_engine": jobType == CanonicalInvestigationJobType,
		"joined_existing_job": !created, "charged": created,
	})
}

func (h *Handler) GetWeb3Job(w http.ResponseWriter, r *http.Request) {
	if h.JobStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "job service unavailable"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := lastCanonicalJobPathSegment(r.URL.Path)
	if id == "" {
		if canonicalHistoryCollectionPath(r.URL.Path) {
			h.CustomerInvestigationHistory(w, r)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	job, err := h.JobStore.Get(r.Context(), id, claims.Sub)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "job read failed"})
		return
	}
	resp := map[string]any{
		"id": job.ID, "job_type": job.Type, "status": job.Status,
		"network": job.Network, "target": job.Target, "progress": job.Progress,
		"attempts": job.Attempts, "queued_at": job.QueuedAt, "updated_at": job.UpdatedAt,
	}
	if len(job.ResultPayload) > 0 && string(job.ResultPayload) != "null" {
		var raw any
		if json.Unmarshal(job.ResultPayload, &raw) == nil {
			resp["result"] = raw
		}
	}
	if job.ErrorCode != "" {
		resp["error_code"] = job.ErrorCode
		resp["error_message"] = job.ErrorMessage
	}
	writeJSON(w, http.StatusOK, resp)
}

func canonicalHistoryCollectionPath(path string) bool {
	return strings.TrimSuffix(strings.TrimSpace(path), "/") == "/api/v1/radar/jobs"
}

func web3JobTypeFromPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "api/v1/radar/jobs" {
		return CanonicalInvestigationJobType
	}
	value := strings.TrimPrefix(path, "api/jobs/")
	switch strings.Trim(value, "/") {
	case "token-scan":
		return legacyTokenScanJobType
	case "wallet-score":
		return "wallet_score"
	case "tx-decode":
		return "tx_decode"
	default:
		return ""
	}
}

func defaultNetworkForJob(jobType string) string {
	if jobType == legacyTokenScanJobType || jobType == CanonicalInvestigationJobType {
		return "solana-mainnet"
	}
	return "solana-devnet"
}
