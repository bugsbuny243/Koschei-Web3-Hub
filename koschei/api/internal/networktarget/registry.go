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
	ID              string `json:"id"`
	Name            string `json:"name"`
	Family          string `json:"family"`
	AddressFormat   string `json:"address_format"`
	CollectorStatus string `json:"collector_status"`
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
func Catalog() []Network {
	return []Network{
		{ID: "solana-mainnet", Name: "Solana", Family: "solana", AddressFormat: "base58-32", CollectorStatus: "existing"},
		{ID: "ethereum-mainnet", Name: "Ethereum", Family: "evm", AddressFormat: "hex-20", CollectorStatus: "probe_ready"},
		{ID: "base-mainnet", Name: "Base", Family: "evm", AddressFormat: "hex-20", CollectorStatus: "probe_ready"},
		{ID: "arbitrum-mainnet", Name: "Arbitrum", Family: "evm", AddressFormat: "hex-20", CollectorStatus: "probe_ready"},
		{ID: "optimism-mainnet", Name: "Optimism", Family: "evm", AddressFormat: "hex-20", CollectorStatus: "probe_ready"},
		{ID: "bitcoin-mainnet", Name: "Bitcoin", Family: "utxo", AddressFormat: "bitcoin-mainnet", CollectorStatus: "probe_ready"},
	}
}

var evmAddress = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

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
	var selected *Network
	for _, network := range Catalog() {
		if network.ID == networkID {
			copy := network
			selected = &copy
			break
		}
	}
	if selected == nil {
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
		Network:           *selected,
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
