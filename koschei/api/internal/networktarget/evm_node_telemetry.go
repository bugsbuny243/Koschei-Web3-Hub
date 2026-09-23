package networktarget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type EVMNodeTelemetryResult struct {
	SchemaVersion    string                      `json:"schema_version"`
	Observation      NetworkTelemetryObservation `json:"observation"`
	ChainID          string                      `json:"chain_id"`
	ExpectedChainID  string                      `json:"expected_chain_id"`
	ClientVersion    string                      `json:"client_version"`
	PeerCount        uint64                      `json:"peer_count"`
	HeadBlock        uint64                      `json:"head_block"`
	Syncing          bool                        `json:"syncing"`
	EndpointScope    string                      `json:"endpoint_scope"`
	AnalysisPerformed bool                       `json:"analysis_performed"`
	LiveAvailability string                      `json:"live_availability"`
}

// ProbeEVMNodeTelemetry observes one configured execution RPC endpoint.
// Peer count, client version and sync state describe only that endpoint; they
// must never be presented as network-wide node counts or validator coverage.
func ProbeEVMNodeTelemetry(ctx context.Context, client *http.Client, endpoint, networkID string, observedAt time.Time) (EVMNodeTelemetryResult, error) {
	network, ok := LookupNetwork(networkID)
	if !ok || network.Family != "evm" {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_telemetry_unsupported_network")
	}
	expectedChainID, ok := ExpectedEVMChainID(networkID)
	if !ok {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_telemetry_chain_id_unknown")
	}
	if observedAt.IsZero() {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_telemetry_observed_at_required")
	}
	parsed, err := validateEVMNodeTelemetryEndpoint(endpoint)
	if err != nil {
		return EVMNodeTelemetryResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	chainID, err := evmRPCString(ctx, client, endpoint, 101, "eth_chainId", nil)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	clientVersion, err := evmRPCString(ctx, client, endpoint, 102, "web3_clientVersion", nil)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_client_version_unavailable: %w", err)
	}
	clientVersion = strings.TrimSpace(clientVersion)
	if clientVersion == "" || len(clientVersion) > 256 {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_client_version_invalid")
	}

	peerRaw, err := evmRPCString(ctx, client, endpoint, 103, "net_peerCount", nil)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_peer_count_unavailable: %w", err)
	}
	peerCount, err := parseEVMQuantity(peerRaw)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_peer_count_invalid")
	}

	headRaw, err := evmRPCString(ctx, client, endpoint, 104, "eth_blockNumber", nil)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_head_unavailable: %w", err)
	}
	headBlock, err := parseEVMQuantity(headRaw)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_head_invalid")
	}

	syncRaw, err := evmNodeTelemetryRPCRaw(ctx, client, endpoint, 105, "eth_syncing", nil)
	if err != nil {
		return EVMNodeTelemetryResult{}, fmt.Errorf("evm_node_sync_state_unavailable: %w", err)
	}
	syncing, err := parseEVMSyncing(syncRaw)
	if err != nil {
		return EVMNodeTelemetryResult{}, err
	}

	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      networkID,
		SubjectKind:    "node",
		SubjectID:      "rpc-endpoint:" + strings.ToLower(parsed.Hostname()),
		Source:         "evm-json-rpc:" + strings.ToLower(parsed.Hostname()),
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
		ClientFamily:   clientVersion,
	})
	if err != nil {
		return EVMNodeTelemetryResult{}, err
	}

	return EVMNodeTelemetryResult{
		SchemaVersion:     NetworkTelemetrySchemaVersion,
		Observation:       observation,
		ChainID:           chainID,
		ExpectedChainID:   expectedChainID,
		ClientVersion:     clientVersion,
		PeerCount:         peerCount,
		HeadBlock:         headBlock,
		Syncing:           syncing,
		EndpointScope:     "single_rpc_endpoint_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, nil
}

func validateEVMNodeTelemetryEndpoint(endpoint string) (*url.URL, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	return parsed, nil
}

func evmNodeTelemetryRPCRaw(ctx context.Context, client *http.Client, endpoint string, id int, method string, params []any) (json.RawMessage, error) {
	payload, err := json.Marshal(evmRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: evmProbeResponseLimit + 1}
	var decoded evmRPCResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return nil, err
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("rpc_response_too_large")
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 {
		return nil, fmt.Errorf("rpc_response_invalid")
	}
	return decoded.Result, nil
}

func parseEVMQuantity(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "0x") || len(value) <= 2 {
		return 0, fmt.Errorf("evm_quantity_invalid")
	}
	return strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
}

func parseEVMSyncing(raw json.RawMessage) (bool, error) {
	var synced bool
	if err := json.Unmarshal(raw, &synced); err == nil {
		return synced, nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || len(object) == 0 {
		return false, fmt.Errorf("evm_node_sync_state_invalid")
	}
	return true, nil
}
