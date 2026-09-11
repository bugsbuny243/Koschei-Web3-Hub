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
	"strings"
	"time"
)

const evmProbeResponseLimit = 256 * 1024

type EVMProbeResult struct {
	SchemaVersion     string     `json:"schema_version"`
	Resolution        Resolution `json:"resolution"`
	ChainID           string     `json:"chain_id"`
	ExpectedChainID   string     `json:"expected_chain_id"`
	ContractCodeState string     `json:"contract_code_state"`
	ContractCodeHash  string     `json:"contract_code_sha256,omitempty"`
	AnalysisPerformed bool       `json:"analysis_performed"`
	EvidenceStatus    string     `json:"evidence_status"`
	LiveAvailability  string     `json:"live_availability"`
}

type evmRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type evmRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func ExpectedEVMChainID(networkID string) (string, bool) {
	switch strings.TrimSpace(networkID) {
	case "ethereum-mainnet":
		return "0x1", true
	case "base-mainnet":
		return "0x2105", true
	case "arbitrum-mainnet":
		return "0xa4b1", true
	case "optimism-mainnet":
		return "0xa", true
	default:
		return "", false
	}
}

// ProbeEVM performs a narrow read-only network check. It verifies that the
// configured RPC endpoint belongs to the requested chain before asking whether
// bytecode is currently observable at the address. Absence of bytecode is not
// treated as proof that an account exists or is safe.
func ProbeEVM(ctx context.Context, client *http.Client, endpoint string, resolution Resolution) (EVMProbeResult, error) {
	expectedChainID, ok := ExpectedEVMChainID(resolution.Network.ID)
	if !ok || resolution.Network.Family != "evm" || !resolution.SyntaxValid {
		return EVMProbeResult{}, fmt.Errorf("evm_probe_unsupported_target")
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return EVMProbeResult{}, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	chainID, err := evmRPCString(ctx, client, endpoint, 1, "eth_chainId", nil)
	if err != nil {
		return EVMProbeResult{}, fmt.Errorf("evm_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMProbeResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	code, err := evmRPCString(ctx, client, endpoint, 2, "eth_getCode", []any{strings.ToLower(resolution.Address), "latest"})
	if err != nil {
		return EVMProbeResult{}, fmt.Errorf("evm_contract_code_unavailable: %w", err)
	}
	code = strings.ToLower(strings.TrimSpace(code))
	if !strings.HasPrefix(code, "0x") {
		return EVMProbeResult{}, fmt.Errorf("evm_contract_code_invalid")
	}
	state := "no_contract_code_observed"
	codeHash := ""
	if code != "0x" && code != "0x0" {
		encoded := strings.TrimPrefix(code, "0x")
		if len(encoded)%2 != 0 {
			encoded = "0" + encoded
		}
		raw, decodeErr := hex.DecodeString(encoded)
		if decodeErr != nil {
			return EVMProbeResult{}, fmt.Errorf("evm_contract_code_invalid")
		}
		sum := sha256.Sum256(raw)
		codeHash = hex.EncodeToString(sum[:])
		state = "contract_code_observed"
	}

	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	return EVMProbeResult{
		SchemaVersion:     SchemaVersion,
		Resolution:        resolution,
		ChainID:           chainID,
		ExpectedChainID:   expectedChainID,
		ContractCodeState: state,
		ContractCodeHash:  codeHash,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}, nil
}

func evmRPCString(ctx context.Context, client *http.Client, endpoint string, id int, method string, params []any) (string, error) {
	payload, err := json.Marshal(evmRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("rpc_status_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: evmProbeResponseLimit + 1}
	var decoded evmRPCResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return "", err
	}
	if limited.N <= 0 {
		return "", fmt.Errorf("rpc_response_too_large")
	}
	if decoded.JSONRPC != "2.0" || decoded.ID != id || decoded.Error != nil || len(decoded.Result) == 0 {
		return "", fmt.Errorf("rpc_response_invalid")
	}
	var result string
	if err := json.Unmarshal(decoded.Result, &result); err != nil || strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("rpc_result_invalid")
	}
	return result, nil
}
