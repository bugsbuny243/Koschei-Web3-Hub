package networktarget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const bitcoinCoreTelemetryResponseLimit = 256 * 1024

type BitcoinCoreNodeTelemetryResult struct {
	SchemaVersion        string                      `json:"schema_version"`
	Observation          NetworkTelemetryObservation `json:"observation"`
	ClientVersion        string                      `json:"client_version"`
	ProtocolVersion      int                         `json:"protocol_version"`
	Connections          int                         `json:"connections"`
	NetworkActive        bool                        `json:"network_active"`
	Blocks               int64                       `json:"blocks"`
	Headers              int64                       `json:"headers"`
	VerificationProgress float64                     `json:"verification_progress"`
	InitialBlockDownload bool                        `json:"initial_block_download"`
	EndpointScope        string                      `json:"endpoint_scope"`
	AnalysisPerformed    bool                        `json:"analysis_performed"`
	LiveAvailability     string                      `json:"live_availability"`
}

type bitcoinCoreRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type bitcoinCoreRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  any             `json:"error"`
	ID     int             `json:"id"`
}

type bitcoinCoreNetworkInfo struct {
	Version         int    `json:"version"`
	Subversion      string `json:"subversion"`
	ProtocolVersion int    `json:"protocolversion"`
	Connections     int    `json:"connections"`
	NetworkActive   bool   `json:"networkactive"`
}

type bitcoinCoreBlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	VerificationProgress float64 `json:"verificationprogress"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
}

// ProbeBitcoinCoreNodeTelemetry observes one explicitly configured Bitcoin Core
// JSON-RPC endpoint. Connections and client version describe only that node.
// Miner/hash-rate distribution is intentionally outside this probe.
func ProbeBitcoinCoreNodeTelemetry(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (BitcoinCoreNodeTelemetryResult, error) {
	if observedAt.IsZero() {
		return BitcoinCoreNodeTelemetryResult{}, fmt.Errorf("bitcoin_core_telemetry_observed_at_required")
	}
	parsed, err := validateBitcoinCoreTelemetryEndpoint(endpoint)
	if err != nil {
		return BitcoinCoreNodeTelemetryResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	var networkInfo bitcoinCoreNetworkInfo
	if err := bitcoinCoreRPC(ctx, client, endpoint, 201, "getnetworkinfo", &networkInfo); err != nil {
		return BitcoinCoreNodeTelemetryResult{}, fmt.Errorf("bitcoin_core_network_info_unavailable: %w", err)
	}
	var blockchainInfo bitcoinCoreBlockchainInfo
	if err := bitcoinCoreRPC(ctx, client, endpoint, 202, "getblockchaininfo", &blockchainInfo); err != nil {
		return BitcoinCoreNodeTelemetryResult{}, fmt.Errorf("bitcoin_core_blockchain_info_unavailable: %w", err)
	}

	clientVersion := strings.TrimSpace(networkInfo.Subversion)
	if clientVersion == "" || len(clientVersion) > 256 || networkInfo.Version < 0 || networkInfo.ProtocolVersion < 0 || networkInfo.Connections < 0 {
		return BitcoinCoreNodeTelemetryResult{}, fmt.Errorf("bitcoin_core_network_info_invalid")
	}
	if blockchainInfo.Chain != "main" || blockchainInfo.Blocks < 0 || blockchainInfo.Headers < 0 || blockchainInfo.VerificationProgress < 0 || blockchainInfo.VerificationProgress > 1 {
		return BitcoinCoreNodeTelemetryResult{}, fmt.Errorf("bitcoin_core_blockchain_info_invalid")
	}

	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "bitcoin-mainnet",
		SubjectKind:    "node",
		SubjectID:      "bitcoin-core-endpoint:" + strings.ToLower(parsed.Hostname()),
		Source:         "bitcoin-core-json-rpc:" + strings.ToLower(parsed.Hostname()),
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
		ClientFamily:   clientVersion,
	})
	if err != nil {
		return BitcoinCoreNodeTelemetryResult{}, err
	}

	return BitcoinCoreNodeTelemetryResult{
		SchemaVersion:        NetworkTelemetrySchemaVersion,
		Observation:          observation,
		ClientVersion:        clientVersion,
		ProtocolVersion:      networkInfo.ProtocolVersion,
		Connections:          networkInfo.Connections,
		NetworkActive:        networkInfo.NetworkActive,
		Blocks:               blockchainInfo.Blocks,
		Headers:              blockchainInfo.Headers,
		VerificationProgress: blockchainInfo.VerificationProgress,
		InitialBlockDownload: blockchainInfo.InitialBlockDownload,
		EndpointScope:        "single_bitcoin_core_node_only",
		AnalysisPerformed:    true,
		LiveAvailability:     "checked",
	}, nil
}

func validateBitcoinCoreTelemetryEndpoint(endpoint string) (*url.URL, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("bitcoin_core_endpoint_invalid")
	}
	return parsed, nil
}

func bitcoinCoreRPC(ctx context.Context, client *http.Client, endpoint string, id int, method string, target any) error {
	payload, err := json.Marshal(bitcoinCoreRPCRequest{JSONRPC: "1.0", ID: id, Method: method, Params: []any{}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: bitcoinCoreTelemetryResponseLimit + 1}
	var decoded bitcoinCoreRPCResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("rpc_response_too_large")
	}
	if decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 {
		return fmt.Errorf("rpc_response_invalid")
	}
	if err := json.Unmarshal(decoded.Result, target); err != nil {
		return fmt.Errorf("rpc_result_invalid")
	}
	return nil
}