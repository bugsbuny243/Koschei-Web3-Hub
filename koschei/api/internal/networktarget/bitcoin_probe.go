package networktarget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	bitcoinProbeResponseLimit   = 256 * 1024
	bitcoinMainnetGenesisHash   = "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
)

type bitcoinAddressStats struct {
	FundedTXOCount int64 `json:"funded_txo_count"`
	FundedTXOSum   int64 `json:"funded_txo_sum"`
	SpentTXOCount  int64 `json:"spent_txo_count"`
	SpentTXOSum    int64 `json:"spent_txo_sum"`
	TXCount        int64 `json:"tx_count"`
}

type bitcoinAddressResponse struct {
	Address      string              `json:"address"`
	ChainStats   bitcoinAddressStats `json:"chain_stats"`
	MempoolStats bitcoinAddressStats `json:"mempool_stats"`
}

type BitcoinProbeResult struct {
	SchemaVersion       string     `json:"schema_version"`
	Resolution          Resolution `json:"resolution"`
	GenesisHash         string     `json:"genesis_hash"`
	ExpectedGenesisHash string     `json:"expected_genesis_hash"`
	ActivityState       string     `json:"activity_state"`
	ConfirmedTXCount    int64      `json:"confirmed_tx_count"`
	MempoolTXCount      int64      `json:"mempool_tx_count"`
	FundedSats          int64      `json:"funded_sats"`
	SpentSats           int64      `json:"spent_sats"`
	AnalysisPerformed   bool       `json:"analysis_performed"`
	EvidenceStatus      string     `json:"evidence_status"`
	LiveAvailability    string     `json:"live_availability"`
}

// ProbeBitcoin performs a narrow read-only observation through an
// Esplora-compatible HTTPS endpoint. It verifies the mainnet genesis hash before
// accepting address activity. No balance, ownership, legitimacy or safety
// conclusion is derived from these observations.
func ProbeBitcoin(ctx context.Context, client *http.Client, endpoint string, resolution Resolution) (BitcoinProbeResult, error) {
	if resolution.Network.ID != "bitcoin-mainnet" || resolution.Network.Family != "utxo" || !resolution.SyntaxValid {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_probe_unsupported_target")
	}
	baseURL, err := validateBitcoinEsploraEndpoint(endpoint)
	if err != nil {
		return BitcoinProbeResult{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	genesisHash, err := bitcoinEsploraText(ctx, client, baseURL+"/block-height/0")
	if err != nil {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_genesis_unavailable: %w", err)
	}
	genesisHash = strings.ToLower(strings.TrimSpace(genesisHash))
	if genesisHash != bitcoinMainnetGenesisHash {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_esplora_network_mismatch")
	}

	var address bitcoinAddressResponse
	if err := bitcoinEsploraJSON(ctx, client, baseURL+"/address/"+url.PathEscape(resolution.Address), &address); err != nil {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_address_activity_unavailable: %w", err)
	}
	if strings.TrimSpace(address.Address) != "" && address.Address != resolution.Address {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_address_response_mismatch")
	}
	if !validBitcoinStats(address.ChainStats) || !validBitcoinStats(address.MempoolStats) {
		return BitcoinProbeResult{}, fmt.Errorf("bitcoin_address_activity_invalid")
	}

	activityState := "no_activity_observed"
	if address.ChainStats.TXCount > 0 || address.MempoolStats.TXCount > 0 || address.ChainStats.FundedTXOCount > 0 || address.MempoolStats.FundedTXOCount > 0 {
		activityState = "activity_observed"
	}

	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	return BitcoinProbeResult{
		SchemaVersion:       SchemaVersion,
		Resolution:          resolution,
		GenesisHash:         genesisHash,
		ExpectedGenesisHash: bitcoinMainnetGenesisHash,
		ActivityState:       activityState,
		ConfirmedTXCount:    address.ChainStats.TXCount,
		MempoolTXCount:      address.MempoolStats.TXCount,
		FundedSats:          address.ChainStats.FundedTXOSum + address.MempoolStats.FundedTXOSum,
		SpentSats:           address.ChainStats.SpentTXOSum + address.MempoolStats.SpentTXOSum,
		AnalysisPerformed:   true,
		EvidenceStatus:      "observed",
		LiveAvailability:    "checked",
	}, nil
}

func validateBitcoinEsploraEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("bitcoin_esplora_endpoint_invalid")
	}
	return strings.TrimRight(endpoint, "/"), nil
}

func validBitcoinStats(stats bitcoinAddressStats) bool {
	return stats.FundedTXOCount >= 0 && stats.FundedTXOSum >= 0 && stats.SpentTXOCount >= 0 && stats.SpentTXOSum >= 0 && stats.TXCount >= 0
}

func bitcoinEsploraText(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("esplora_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: 256 + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if limited.N <= 0 {
		return "", fmt.Errorf("esplora_response_too_large")
	}
	value := strings.TrimSpace(string(body))
	if value == "" {
		return "", fmt.Errorf("esplora_response_invalid")
	}
	return value, nil
}

func bitcoinEsploraJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("esplora_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: bitcoinProbeResponseLimit + 1}
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("esplora_response_too_large")
	}
	return nil
}
