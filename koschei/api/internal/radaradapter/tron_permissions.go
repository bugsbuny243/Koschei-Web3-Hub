package radaradapter

import (
	"encoding/json"
	"errors"
	"fmt"
)

// TronPermissionFinding describes an account permission's signing threshold.
// It is a read-only observation, not a compromise or safety verdict.
type TronPermissionFinding struct {
	ID string
	Type int
	Threshold int64
	TotalWeight int64
	KeyCount int
	ThresholdReachable bool
}

type tronPermissionResponse struct {
	OwnerPermission *tronPermissionJSON `json:"owner_permission"`
	ActivePermission []tronPermissionJSON `json:"active_permission"`
	WitnessPermission *tronPermissionJSON `json:"witness_permission"`
}
type tronPermissionJSON struct {
	ID int `json:"id"`
	Type int `json:"type"`
	Threshold int64 `json:"threshold"`
	Keys []struct {
		Address string `json:"address"`
		Weight int64 `json:"weight"`
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
		if permission.Threshold <= 0 {
			return nil, errors.New("tron permission threshold must be positive")
		}
		var total int64
		for _, key := range permission.Keys {
			if key.Address == "" || key.Weight <= 0 {
				return nil, errors.New("tron permission key is incomplete")
			}
			if total > int64(^uint64(0)>>1)-key.Weight {
				return nil, errors.New("tron permission weight overflow")
			}
			total += key.Weight
		}
		findings = append(findings, TronPermissionFinding{
			ID: fmt.Sprintf("%d", permission.ID),
			Type: permission.Type,
			Threshold: permission.Threshold,
			TotalWeight: total,
			KeyCount: len(permission.Keys),
			ThresholdReachable: total >= permission.Threshold,
		})
	}
	return findings, nil
}
