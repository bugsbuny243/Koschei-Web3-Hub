package networktarget

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ethereumBeaconTelemetryResponseLimit = 256 * 1024

var ethereumBeaconRoot = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

type EthereumBeaconCheckpoint struct {
	Epoch uint64 `json:"epoch"`
	Root  string `json:"root"`
}

type EthereumBeaconTelemetryResult struct {
	SchemaVersion            string                      `json:"schema_version"`
	Observation              NetworkTelemetryObservation `json:"observation"`
	ClientVersion            string                      `json:"client_version"`
	VersionResponseSHA256    string                      `json:"version_response_sha256,omitempty"`
	ConnectedPeers           uint64                      `json:"connected_peers"`
	PeerCountResponseSHA256  string                      `json:"peer_count_response_sha256,omitempty"`
	HeadSlot                 uint64                      `json:"head_slot"`
	SyncDistance             uint64                      `json:"sync_distance"`
	IsSyncing                bool                        `json:"is_syncing"`
	IsOptimistic             bool                        `json:"is_optimistic"`
	ExecutionLayerOffline    bool                        `json:"execution_layer_offline"`
	SyncingResponseSHA256    string                      `json:"syncing_response_sha256,omitempty"`
	PreviousJustified        EthereumBeaconCheckpoint    `json:"previous_justified"`
	CurrentJustified         EthereumBeaconCheckpoint    `json:"current_justified"`
	Finalized                EthereumBeaconCheckpoint    `json:"finalized"`
	CheckpointStateFinalized bool                        `json:"checkpoint_state_finalized"`
	FinalityResponseSHA256   string                      `json:"finality_response_sha256,omitempty"`
	EndpointScope            string                      `json:"endpoint_scope"`
	AnalysisPerformed        bool                        `json:"analysis_performed"`
	LiveAvailability         string                      `json:"live_availability"`
}

type ethereumBeaconVersionResponse struct {
	Data struct {
		Version string `json:"version"`
	} `json:"data"`
}

type ethereumBeaconPeerCountResponse struct {
	Data struct {
		Disconnected  string `json:"disconnected"`
		Connecting    string `json:"connecting"`
		Connected     string `json:"connected"`
		Disconnecting string `json:"disconnecting"`
	} `json:"data"`
}

type ethereumBeaconSyncingResponse struct {
	Data struct {
		HeadSlot     string `json:"head_slot"`
		SyncDistance string `json:"sync_distance"`
		IsSyncing    bool   `json:"is_syncing"`
		IsOptimistic bool   `json:"is_optimistic"`
		ELOffline    bool   `json:"el_offline"`
	} `json:"data"`
}

type ethereumBeaconCheckpointWire struct {
	Epoch string `json:"epoch"`
	Root  string `json:"root"`
}

type ethereumBeaconFinalityResponse struct {
	ExecutionOptimistic bool `json:"execution_optimistic"`
	Finalized           bool `json:"finalized"`
	Data                struct {
		PreviousJustified ethereumBeaconCheckpointWire `json:"previous_justified"`
		CurrentJustified  ethereumBeaconCheckpointWire `json:"current_justified"`
		Finalized         ethereumBeaconCheckpointWire `json:"finalized"`
	} `json:"data"`
}

// ProbeEthereumBeaconTelemetry observes one configured Ethereum Beacon Node
// REST endpoint. Peer count, client version and sync state describe only that
// endpoint. Finality checkpoints are consensus-chain evidence, but this probe
// does not claim global validator population or stake distribution.
func ProbeEthereumBeaconTelemetry(ctx context.Context, client *http.Client, endpoint string, observedAt time.Time) (EthereumBeaconTelemetryResult, error) {
	if observedAt.IsZero() {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_observed_at_required")
	}
	baseURL, endpointHost, err := validateEthereumBeaconEndpoint(endpoint)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	var version ethereumBeaconVersionResponse
	versionResponseSHA256, err := ethereumBeaconGETJSONWithDigest(ctx, client, baseURL+"/eth/v1/node/version", &version)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_version_unavailable: %w", err)
	}
	clientVersion := strings.TrimSpace(version.Data.Version)
	if clientVersion == "" || len(clientVersion) > 256 {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_version_invalid")
	}

	var peers ethereumBeaconPeerCountResponse
	peerCountResponseSHA256, err := ethereumBeaconGETJSONWithDigest(ctx, client, baseURL+"/eth/v1/node/peer_count", &peers)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_peer_count_unavailable: %w", err)
	}
	connectedPeers, err := parseEthereumBeaconUint64(peers.Data.Connected)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_peer_count_invalid")
	}
	for _, raw := range []string{peers.Data.Disconnected, peers.Data.Connecting, peers.Data.Disconnecting} {
		if _, err := parseEthereumBeaconUint64(raw); err != nil {
			return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_peer_count_invalid")
		}
	}

	var syncing ethereumBeaconSyncingResponse
	syncingResponseSHA256, err := ethereumBeaconGETJSONWithDigest(ctx, client, baseURL+"/eth/v1/node/syncing", &syncing)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_sync_unavailable: %w", err)
	}
	headSlot, err := parseEthereumBeaconUint64(syncing.Data.HeadSlot)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_head_slot_invalid")
	}
	syncDistance, err := parseEthereumBeaconUint64(syncing.Data.SyncDistance)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_sync_distance_invalid")
	}

	var finality ethereumBeaconFinalityResponse
	finalityResponseSHA256, err := ethereumBeaconGETJSONWithDigest(ctx, client, baseURL+"/eth/v1/beacon/states/head/finality_checkpoints", &finality)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_finality_unavailable: %w", err)
	}
	previousJustified, err := normalizeEthereumBeaconCheckpoint(finality.Data.PreviousJustified)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_previous_justified_invalid")
	}
	currentJustified, err := normalizeEthereumBeaconCheckpoint(finality.Data.CurrentJustified)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_current_justified_invalid")
	}
	finalizedCheckpoint, err := normalizeEthereumBeaconCheckpoint(finality.Data.Finalized)
	if err != nil {
		return EthereumBeaconTelemetryResult{}, fmt.Errorf("ethereum_beacon_finalized_invalid")
	}

	observation, err := NormalizeNetworkTelemetry(NetworkTelemetryInput{
		NetworkID:      "ethereum-mainnet",
		SubjectKind:    "node",
		SubjectID:      "ethereum-beacon-endpoint:" + endpointHost,
		Source:         "ethereum-beacon-api:" + endpointHost,
		ObservedAt:     observedAt,
		EvidenceStatus: "observed",
		ClientFamily:   clientVersion,
	})
	if err != nil {
		return EthereumBeaconTelemetryResult{}, err
	}

	return EthereumBeaconTelemetryResult{
		SchemaVersion:            NetworkTelemetrySchemaVersion,
		Observation:              observation,
		ClientVersion:            clientVersion,
		VersionResponseSHA256:    versionResponseSHA256,
		ConnectedPeers:           connectedPeers,
		PeerCountResponseSHA256:  peerCountResponseSHA256,
		HeadSlot:                 headSlot,
		SyncDistance:             syncDistance,
		IsSyncing:                syncing.Data.IsSyncing,
		IsOptimistic:             syncing.Data.IsOptimistic || finality.ExecutionOptimistic,
		ExecutionLayerOffline:    syncing.Data.ELOffline,
		SyncingResponseSHA256:    syncingResponseSHA256,
		PreviousJustified:        previousJustified,
		CurrentJustified:         currentJustified,
		Finalized:                finalizedCheckpoint,
		CheckpointStateFinalized: finality.Finalized,
		FinalityResponseSHA256:   finalityResponseSHA256,
		EndpointScope:            "single_beacon_endpoint_plus_chain_checkpoints",
		AnalysisPerformed:        true,
		LiveAvailability:         "checked",
	}, nil
}

func validateEthereumBeaconEndpoint(endpoint string) (string, string, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("ethereum_beacon_endpoint_invalid")
	}
	return strings.TrimRight(endpoint, "/"), strings.ToLower(parsed.Hostname()), nil
}

func ethereumBeaconGETJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	_, err := ethereumBeaconGETJSONWithDigest(ctx, client, endpoint, target)
	return err
}

func ethereumBeaconGETJSONWithDigest(ctx context.Context, client *http.Client, endpoint string, target any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("beacon_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: ethereumBeaconTelemetryResponseLimit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if limited.N <= 0 {
		return "", fmt.Errorf("beacon_response_too_large")
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(target); err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func parseEthereumBeaconUint64(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("uint64_missing")
	}
	return strconv.ParseUint(value, 10, 64)
}

func normalizeEthereumBeaconCheckpoint(value ethereumBeaconCheckpointWire) (EthereumBeaconCheckpoint, error) {
	epoch, err := parseEthereumBeaconUint64(value.Epoch)
	if err != nil {
		return EthereumBeaconCheckpoint{}, err
	}
	root := strings.ToLower(strings.TrimSpace(value.Root))
	if !ethereumBeaconRoot.MatchString(root) {
		return EthereumBeaconCheckpoint{}, fmt.Errorf("checkpoint_root_invalid")
	}
	return EthereumBeaconCheckpoint{Epoch: epoch, Root: root}, nil
}
