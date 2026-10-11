package radaradapter

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TronPermissionFinding describes an account permission's signing threshold.
// It is a read-only observation, not a compromise or safety verdict.
type TronPermissionFinding struct {
	ID                 string
	Operations         string
	Type               int
	Threshold          int64
	TotalWeight        int64
	KeyCount           int
	ThresholdReachable bool
}

type tronPermissionResponse struct {
	OwnerPermission   *tronPermissionJSON  `json:"owner_permission"`
	ActivePermission  []tronPermissionJSON `json:"active_permission"`
	WitnessPermission *tronPermissionJSON  `json:"witness_permission"`
}
type tronPermissionJSON struct {
	ID        int   `json:"id"`
	Type      int   `json:"type"`
	Threshold int64 `json:"threshold"`
	Operations string `json:"operations"`
	Keys      []struct {
		Address string `json:"address"`
		Weight  int64  `json:"weight"`
	} `json:"keys"`
}

// ParseTronAccountPermissions parses Tron account permission fields from a
// getaccount response. It does not infer the legitimacy of any signer.
func ParseTronAccountPermissions(raw []byte) ([]TronPermissionFinding, error) {
	var response tronPermissionResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	permissions := make([]tronPermissionJSON, 0, len(response.ActivePermission)+2)
	if response.OwnerPermission != nil {
		permissions = append(permissions, *response.OwnerPermission)
	}
	permissions = append(permissions, response.ActivePermission...)
	if response.WitnessPermission != nil {
		permissions = append(permissions, *response.WitnessPermission)
	}
	if len(permissions) == 0 {
		return nil, errors.New("tron account response contains no permissions")
	}
	findings := make([]TronPermissionFinding, 0, len(permissions))
	seen := make(map[int]bool, len(permissions))
	for _, permission := range permissions {
		if permission.ID < 0 || seen[permission.ID] {
			return nil, errors.New("invalid or repeated tron permission ID")
		}
		seen[permission.ID] = true
		if permission.Type == 2 && permission.Operations != "" {
		if len(permission.Operations) != 64 {
			return nil, errors.New("invalid tron active permission operations mask length")
		}
		for _, digit := range permission.Operations {
			if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
				return nil, errors.New("invalid tron active permission operations mask encoding")
			}
		}
	}
		if permission.Threshold <= 0 {
			return nil, errors.New("tron permission threshold must be positive")
		}
		var total int64
		seenKeys := make(map[string]bool, len(permission.Keys))
		for _, key := range permission.Keys {
			if !validTronPermissionAddress(key.Address) || key.Weight <= 0 {
				return nil, errors.New("tron permission key is incomplete")
			}
			signerID := canonicalTronPermissionSigner(key.Address)
			if seenKeys[signerID] {
				return nil, errors.New("duplicate tron permission signer")
			}
			seenKeys[signerID] = true
			if total > int64(^uint64(0)>>1)-key.Weight {
				return nil, errors.New("tron permission weight overflow")
			}
			total += key.Weight
		}
		findings = append(findings, TronPermissionFinding{
			ID:                 fmt.Sprintf("%d", permission.ID),
			Operations:         strings.ToLower(permission.Operations),
			Type:               permission.Type,
			Threshold:          permission.Threshold,
			TotalWeight:        total,
			KeyCount:           len(permission.Keys),
			ThresholdReachable: total >= permission.Threshold,
		})
	}
	return findings, nil
}

// Tron account APIs may return a 21-byte hex address (41 prefix) or a
// 34-character Base58Check address. Both encodings are validated locally.
func validTronPermissionAddress(address string) bool {
	return canonicalTronPermissionSigner(address) != ""
}

func canonicalTronPermissionSigner(address string) string {
	if address != strings.TrimSpace(address) {
		return ""
	}
	if len(address) == 42 && strings.HasPrefix(address, "41") {
		for _, char := range address[2:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
				return ""
			}
		}
		return strings.ToLower(address)
	}
	if len(address) == 34 && strings.HasPrefix(address, "T") {
		const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		decoded := make([]byte, 25)
		for _, char := range address {
			index := strings.IndexRune(alphabet, char)
			if index < 0 {
				return ""
			}
			carry := index
			for i := len(decoded) - 1; i >= 0; i-- {
				carry += int(decoded[i]) * 58
				decoded[i] = byte(carry)
				carry >>= 8
			}
			if carry != 0 {
				return ""
			}
		}
		if decoded[0] != 0x41 {
			return ""
		}
		first := sha256.Sum256(decoded[:21])
		second := sha256.Sum256(first[:])
		for i := 0; i < 4; i++ {
			if decoded[21+i] != second[i] {
				return ""
			}
		}
		return fmt.Sprintf("%x", decoded[:21])
	}
	return ""
}
