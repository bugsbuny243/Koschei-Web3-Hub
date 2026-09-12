package networktarget

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	EVMProxyAuthorityNotObserved = "no_standard_proxy_authority_observed"
	EVMProxyAuthorityObserved    = "eip1967_proxy_authority_observed"
)

const (
	eip1967ImplementationSlot = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"
	eip1967BeaconSlot         = "0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50"
	eip1967AdminSlot          = "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"
)

// EVMProxyAuthorityResult reports standard ERC-1967 storage observations only.
// Empty ERC-1967 slots do not prove immutability or absence of upgrade logic;
// contracts may use other proxy or upgrade patterns.
type EVMProxyAuthorityResult struct {
	SchemaVersion     string     `json:"schema_version"`
	Resolution        Resolution `json:"resolution"`
	ChainID           string     `json:"chain_id"`
	ExpectedChainID   string     `json:"expected_chain_id"`
	State             string     `json:"state"`
	Implementation    string     `json:"implementation,omitempty"`
	Admin             string     `json:"admin,omitempty"`
	Beacon            string     `json:"beacon,omitempty"`
	ConflictingSlots  bool       `json:"conflicting_slots"`
	AnalysisPerformed bool       `json:"analysis_performed"`
	EvidenceStatus    string     `json:"evidence_status"`
	LiveAvailability  string     `json:"live_availability"`
}

func ProbeEVMProxyAuthority(ctx context.Context, client *http.Client, endpoint string, resolution Resolution) (EVMProxyAuthorityResult, error) {
	expectedChainID, ok := ExpectedEVMChainID(resolution.Network.ID)
	if !ok || resolution.Network.Family != "evm" || !resolution.SyntaxValid {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_probe_unsupported_target")
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_rpc_endpoint_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	chainID, err := evmRPCString(ctx, client, endpoint, 21, "eth_chainId", nil)
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_chain_id_unavailable: %w", err)
	}
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	if chainID != expectedChainID {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_rpc_network_mismatch")
	}

	address := strings.ToLower(strings.TrimSpace(resolution.Address))
	implementationWord, err := evmRPCString(ctx, client, endpoint, 22, "eth_getStorageAt", []any{address, eip1967ImplementationSlot, "latest"})
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_implementation_slot_unavailable: %w", err)
	}
	adminWord, err := evmRPCString(ctx, client, endpoint, 23, "eth_getStorageAt", []any{address, eip1967AdminSlot, "latest"})
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_admin_slot_unavailable: %w", err)
	}
	beaconWord, err := evmRPCString(ctx, client, endpoint, 24, "eth_getStorageAt", []any{address, eip1967BeaconSlot, "latest"})
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_beacon_slot_unavailable: %w", err)
	}

	implementation, err := evmStorageWordAddress(implementationWord)
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_implementation_slot_invalid")
	}
	admin, err := evmStorageWordAddress(adminWord)
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_admin_slot_invalid")
	}
	beacon, err := evmStorageWordAddress(beaconWord)
	if err != nil {
		return EVMProxyAuthorityResult{}, fmt.Errorf("evm_proxy_beacon_slot_invalid")
	}

	state := EVMProxyAuthorityNotObserved
	if implementation != "" || admin != "" || beacon != "" {
		state = EVMProxyAuthorityObserved
	}
	conflicting := implementation != "" && beacon != ""

	resolution.AnalysisPerformed = true
	resolution.EvidenceStatus = "observed"
	resolution.LiveAvailability = "checked"
	return EVMProxyAuthorityResult{
		SchemaVersion:     SchemaVersion,
		Resolution:        resolution,
		ChainID:           chainID,
		ExpectedChainID:   expectedChainID,
		State:             state,
		Implementation:    implementation,
		Admin:             admin,
		Beacon:            beacon,
		ConflictingSlots:  conflicting,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}, nil
}

func evmStorageWordAddress(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return "", fmt.Errorf("invalid storage word")
	}
	hexPart := value[2:]
	for _, ch := range hexPart {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return "", fmt.Errorf("invalid storage word")
		}
	}
	address := hexPart[len(hexPart)-40:]
	if address == strings.Repeat("0", 40) {
		return "", nil
	}
	return "0x" + address, nil
}
