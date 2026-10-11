package radaradapter

import (
	"encoding/json"
	"errors"
	"strings"
)

// EVMProxyAuthority is an observation of standard EIP-1967 storage slots.
// A nonzero slot alone does not prove a contract is upgradeable or malicious.
type EVMProxyAuthority struct {
	Implementation string
	Admin string
	Beacon string
}

type evmStorageRPC struct {
	Result string `json:"result"`
	Error *struct {
		Code int `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ParseEVMStorageAddress extracts the low 20 bytes from an eth_getStorageAt
// JSON-RPC response. Zero slots are represented by empty strings.
func ParseEVMStorageAddress(raw []byte) (string, error) {
	var response evmStorageRPC
	if err := json.Unmarshal(raw, &response); err != nil { return "", err }
	if response.Error != nil { return "", errors.New("EVM storage provider returned RPC error") }
	value := strings.TrimPrefix(response.Result, "0x")
	if len(value) != 64 { return "", errors.New("EVM storage slot must contain 32 bytes") }
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return "", errors.New("EVM storage slot contains invalid hex")
		}
	}
	address := strings.ToLower(value[24:])
	if address == strings.Repeat("0", 40) { return "", nil }
	return "0x" + address, nil
}

// ParseEVMProxyAuthority requires all three EIP-1967 slot responses. Missing
// data fails closed instead of implying absent upgrade authority.
func ParseEVMProxyAuthority(implementation, admin, beacon []byte) (EVMProxyAuthority, error) {
	var result EVMProxyAuthority
	var err error
	if result.Implementation, err = ParseEVMStorageAddress(implementation); err != nil { return EVMProxyAuthority{}, err }
	if result.Admin, err = ParseEVMStorageAddress(admin); err != nil { return EVMProxyAuthority{}, err }
	if result.Beacon, err = ParseEVMStorageAddress(beacon); err != nil { return EVMProxyAuthority{}, err }
	return result, nil
}
