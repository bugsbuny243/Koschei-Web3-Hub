// Package networktarget binds address syntax to an explicitly selected network.
// Resolution is offline metadata, never evidence of a safe or existing account.
package networktarget

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const SchemaVersion = "koschei.network-target.v1"

type Network struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Family              string `json:"family"`
	Environment         string `json:"environment"`
	ConsensusFamily     string `json:"consensus_family"`
	AddressFormat       string `json:"address_format"`
	CollectorStatus     string `json:"collector_status"`
	NodeTelemetryStatus string `json:"node_telemetry_status"`
}

type Resolution struct {
	SchemaVersion     string  `json:"schema_version"`
	Network           Network `json:"network"`
	Address           string  `json:"address"`
	CanonicalRef      string  `json:"canonical_ref"`
	SubjectID         string  `json:"subject_id"`
	Classification    string  `json:"classification"`
	SyntaxValid       bool    `json:"syntax_valid"`
	AnalysisPerformed bool    `json:"analysis_performed"`
	EvidenceStatus    string  `json:"evidence_status"`
	LiveAvailability  string  `json:"live_availability"`
}

// Catalog describes collector implementation, not deployment health. A new
// adapter must provide its own evidence before it can produce an ARVIS verdict.
// Runtime configuration/availability is reported separately by the deployment
// catalog and live probe endpoints.
//
// ConsensusFamily is a coarse monitoring category used to select evidence
// dimensions. It is not a claim that networks in the same category have
// equivalent consensus or security properties.
func Catalog() []Network {
	return []Network{
		{ID: "solana-mainnet", Name: "Solana", Family: "solana", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "base58-32", CollectorStatus: "existing", NodeTelemetryStatus: "validator_probe_ready"},
		{ID: "ethereum-mainnet", Name: "Ethereum", Family: "evm", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "execution_and_beacon_probe_ready"},
		{ID: "base-mainnet", Name: "Base", Family: "evm", Environment: "mainnet", ConsensusFamily: "rollup", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_and_finality_probe_ready"},
		{ID: "arbitrum-mainnet", Name: "Arbitrum", Family: "evm", Environment: "mainnet", ConsensusFamily: "rollup", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_and_finality_probe_ready"},
		{ID: "optimism-mainnet", Name: "Optimism", Family: "evm", Environment: "mainnet", ConsensusFamily: "rollup", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_and_finality_probe_ready"},
		{ID: "polygon-mainnet", Name: "Polygon", Family: "evm", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_probe_ready"},
		{ID: "bnb-mainnet", Name: "BNB Smart Chain", Family: "evm", Environment: "mainnet", ConsensusFamily: "proof_of_staked_authority", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_probe_ready"},
		{ID: "avalanche-mainnet", Name: "Avalanche C-Chain", Family: "evm", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "hex-20", CollectorStatus: "probe_ready", NodeTelemetryStatus: "rpc_node_probe_ready"},
		{ID: "bitcoin-mainnet", Name: "Bitcoin", Family: "utxo", Environment: "mainnet", ConsensusFamily: "proof_of_work", AddressFormat: "bitcoin-mainnet", CollectorStatus: "probe_ready", NodeTelemetryStatus: "core_node_and_pow_network_probe_ready"},
		{ID: "sui-mainnet", Name: "Sui", Family: "move", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "move-hex-32-strict", CollectorStatus: "probe_ready", NodeTelemetryStatus: "chain_identity_probe_ready"},
		{ID: "aptos-mainnet", Name: "Aptos", Family: "move", Environment: "mainnet", ConsensusFamily: "proof_of_stake", AddressFormat: "move-hex-32-strict", CollectorStatus: "probe_ready", NodeTelemetryStatus: "ledger_identity_probe_ready"},
		{ID: "cosmoshub-mainnet", Name: "Cosmos Hub", Family: "cosmos", Environment: "mainnet", ConsensusFamily: "cometbft", AddressFormat: "bech32-unimplemented", CollectorStatus: "network_probe_ready", NodeTelemetryStatus: "cometbft_validator_probe_ready"},
		{ID: "osmosis-mainnet", Name: "Osmosis", Family: "cosmos", Environment: "mainnet", ConsensusFamily: "cometbft", AddressFormat: "bech32-unimplemented", CollectorStatus: "network_probe_ready", NodeTelemetryStatus: "cometbft_validator_probe_ready"},
	}
}

func LookupNetwork(networkID string) (Network, bool) {
	networkID = strings.TrimSpace(networkID)
	for _, network := range Catalog() {
		if network.ID == networkID {
			return network, true
		}
	}
	return Network{}, false
}

var evmAddress = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var moveHex32Address = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func solanaAddress(value string) bool {
	if len(value) < 32 || len(value) > 44 {
		return false
	}
	n := new(big.Int)
	base := big.NewInt(58)
	for _, ch := range value {
		digit := strings.IndexRune(base58Alphabet, ch)
		if digit < 0 {
			return false
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(digit)))
	}
	leadingZeros := len(value) - len(strings.TrimLeft(value, "1"))
	return leadingZeros+len(n.Bytes()) == 32
}

func Resolve(networkID, address string) (Resolution, error) {
	if networkID == "" {
		return Resolution{}, fmt.Errorf("network_required")
	}
	selected, ok := LookupNetwork(networkID)
	if !ok {
		return Resolution{}, fmt.Errorf("network_not_registered")
	}
	address = strings.TrimSpace(address)
	if len(address) == 0 || len(address) > 128 {
		return Resolution{}, fmt.Errorf("invalid_address_length")
	}
	canonicalAddress := address
	classification := "address_syntax_only"
	switch selected.AddressFormat {
	case "hex-20":
		if !evmAddress.MatchString(address) {
			return Resolution{}, fmt.Errorf("address_network_format_mismatch")
		}
		canonicalAddress = strings.ToLower(address)
		classification = "evm_hex_syntax_only_checksum_not_verified"
	case "base58-32":
		if !solanaAddress(address) {
			return Resolution{}, fmt.Errorf("address_network_format_mismatch")
		}
	case "move-hex-32-strict":
		if !moveHex32Address.MatchString(address) {
			return Resolution{}, fmt.Errorf("address_network_format_mismatch")
		}
		canonicalAddress = strings.ToLower(address)
		classification = "move_hex_32_canonical_syntax_only"
	case "bitcoin-mainnet":
		var ok bool
		canonicalAddress, classification, ok = bitcoinMainnetAddress(address)
		if !ok {
			return Resolution{}, fmt.Errorf("address_network_format_mismatch")
		}
	default:
		return Resolution{}, fmt.Errorf("network_address_parser_not_implemented")
	}
	canonical := selected.ID + ":" + canonicalAddress
	digest := sha256.Sum256([]byte(canonical))
	return Resolution{
		SchemaVersion:     SchemaVersion,
		Network:           selected,
		Address:           address,
		CanonicalRef:      canonical,
		SubjectID:         fmt.Sprintf("network-subject:%x", digest),
		Classification:    classification,
		SyntaxValid:       true,
		AnalysisPerformed: false,
		EvidenceStatus:    "unknown",
		LiveAvailability:  "not_checked",
	}, nil
}
