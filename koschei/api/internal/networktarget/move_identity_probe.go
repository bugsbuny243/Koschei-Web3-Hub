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

const moveIdentityResponseLimit = 128 * 1024
const suiMainnetChainIdentifierBase58 = "4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"
const aptosMainnetChainID = 1

type SuiMainnetIdentityProbeResult struct {
	SchemaVersion     string                      `json:"schema_version"`
	Observation       NetworkTelemetryObservation `json:"observation"`
	ChainIdentifier   string                      `json:"chain_identifier"`
	EndpointScope     string                      `json:"endpoint_scope"`
	AnalysisPerformed bool                        `json:"analysis_performed"`
	LiveAvailability  string                      `json:"live_availability"`
}

type AptosMainnetIdentityProbeResult struct {
	SchemaVersion     string                      `json:"schema_version"`
	Observation       NetworkTelemetryObservation `json:"observation"`
	ChainID           uint8                       `json:"chain_id"`
	Epoch             uint64                      `json:"epoch"`
	LedgerVersion     uint64                      `json:"ledger_version"`
	LedgerTimestamp   uint64                      `json:"ledger_timestamp"`
	BlockHeight       uint64                      `json:"block_height"`
	NodeRole          string                      `json:"node_role"`
	GitHash           string                      `json:"git_hash,omitempty"`
	EndpointScope     string                      `json:"endpoint_scope"`
	AnalysisPerformed bool                        `json:"analysis_performed"`
	LiveAvailability  string                      `json:"live_availability"`
}

type suiGraphQLIdentityResponse struct {
	Data struct {
		ChainIdentifier string `json:"chainIdentifier"`
	} `json:"data"`
	Errors []json.RawMessage `json:"errors,omitempty"`
}

type aptosLedgerIndexResponse struct {
	ChainID         uint8  `json:"chain_id"`
	Epoch           string `json:"epoch"`
	LedgerVersion   string `json:"ledger_version"`
	LedgerTimestamp string `json:"ledger_timestamp"`
	BlockHeight     string `json:"block_height"`
	NodeRole        string `json:"node_role"`
	GitHash         string `json:"git_hash,omitempty"`
}

// ProbeSuiMainnetIdentity verifies the configured endpoint against Sui's
// canonical mainnet chain identifier. It proves network identity only; it does
// not claim validator, stake or account safety evidence.
func ProbeSuiMainnetIdentity(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (SuiMainnetIdentityProbeResult, error) {
	if observedAt.IsZero() {
		return SuiMainnetIdentityProbeResult{}, fmt.Errorf("sui_identity_observed_at_required")
	}
	parsed, err := validateMoveIdentityEndpoint(endpoint)
	if err != nil {
		return SuiMainnetIdentityProbeResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	payload, err := json.Marshal(map[string]string{"query": "{ chainIdentifier }"})
	if err != nil {
		return SuiMainnetIdentityProbeResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return SuiMainnetIdentityProbeResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var response suiGraphQLIdentityResponse
	if err := moveIdentityDoJSON(client, req, &response); err != nil {
		return SuiMainnetIdentityProbeResult{}, fmt.Errorf("sui_chain_identity_unavailable: %w", err)
	}
	if len(response.Errors) > 0 {
		return SuiMainnetIdentityProbeResult{}, fmt.Errorf("sui_chain_identity_graphql_error")
	}
	chainIdentifier := strings.TrimSpace(response.Data.ChainIdentifier)
	if chainIdentifier != suiMainnetChainIdentifierBase58 {
		return SuiMainnetIdentityProbeResult{}, fmt.Errorf("sui_mainnet_chain_identity_mismatch")
	}

	host := strings.ToLower(parsed.Hostname())
	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "sui-mainnet",
		SubjectKind:    "network",
		SubjectID:      "sui-mainnet",
		Source:         "sui-graphql-chain-identity:" + host,
		ObservedAt:     observedAt,
		EvidenceStatus: "verified",
	})
	if err != nil {
		return SuiMainnetIdentityProbeResult{}, err
	}

	return SuiMainnetIdentityProbeResult{
		SchemaVersion:     NetworkTelemetrySchemaVersion,
		Observation:       observation,
		ChainIdentifier:   chainIdentifier,
		EndpointScope:     "sui_graphql_chain_identity_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, nil
}

// ProbeAptosMainnetIdentity verifies Aptos mainnet chain_id=1 and records
// bounded ledger freshness metadata from the REST index endpoint.
func ProbeAptosMainnetIdentity(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (AptosMainnetIdentityProbeResult, error) {
	if observedAt.IsZero() {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_identity_observed_at_required")
	}
	parsed, err := validateMoveIdentityEndpoint(endpoint)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, err
	}
	req.Header.Set("Accept", "application/json")

	var response aptosLedgerIndexResponse
	if err := moveIdentityDoJSON(client, req, &response); err != nil {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_ledger_identity_unavailable: %w", err)
	}
	if response.ChainID != aptosMainnetChainID {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_mainnet_chain_identity_mismatch")
	}

	epoch, err := parseMoveIdentityUint64(response.Epoch)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_epoch_invalid")
	}
	ledgerVersion, err := parseMoveIdentityUint64(response.LedgerVersion)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_ledger_version_invalid")
	}
	ledgerTimestamp, err := parseMoveIdentityUint64(response.LedgerTimestamp)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_ledger_timestamp_invalid")
	}
	blockHeight, err := parseMoveIdentityUint64(response.BlockHeight)
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_block_height_invalid")
	}

	nodeRole := strings.TrimSpace(response.NodeRole)
	if nodeRole == "" || len(nodeRole) > 64 {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_node_role_invalid")
	}
	gitHash := strings.TrimSpace(response.GitHash)
	if len(gitHash) > 128 {
		return AptosMainnetIdentityProbeResult{}, fmt.Errorf("aptos_git_hash_invalid")
	}

	host := strings.ToLower(parsed.Hostname())
	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "aptos-mainnet",
		SubjectKind:    "network",
		SubjectID:      "aptos-mainnet",
		Source:         "aptos-rest-ledger-index:" + host,
		ObservedAt:     observedAt,
		EvidenceStatus: "verified",
	})
	if err != nil {
		return AptosMainnetIdentityProbeResult{}, err
	}

	return AptosMainnetIdentityProbeResult{
		SchemaVersion:     NetworkTelemetrySchemaVersion,
		Observation:       observation,
		ChainID:           response.ChainID,
		Epoch:             epoch,
		LedgerVersion:     ledgerVersion,
		LedgerTimestamp:   ledgerTimestamp,
		BlockHeight:       blockHeight,
		NodeRole:          nodeRole,
		GitHash:           gitHash,
		EndpointScope:     "aptos_rest_mainnet_ledger_identity",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, nil
}

func validateMoveIdentityEndpoint(endpoint string) (*url.URL, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("move_identity_endpoint_invalid")
	}
	return parsed, nil
}

func moveIdentityDoJSON(client *http.Client, req *http.Request, target any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: moveIdentityResponseLimit + 1}
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("response_too_large")
	}
	return nil
}

func parseMoveIdentityUint64(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("uint64_missing")
	}
	return strconv.ParseUint(value, 10, 64)
}
