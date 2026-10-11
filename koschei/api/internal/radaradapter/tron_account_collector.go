package radaradapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// TronAccountObservation contains raw-response provenance and parsed permission
// observations. It is not a verified security verdict or a risk score.
type TronAccountObservation struct {
	Network           string
	Address           string
	Provider          string
	RawResponseSHA256 string
	BlockNumber      int64
	Permissions       []TronPermissionFinding
}

// CollectTronMainnetPermissions performs a read-only getaccount request against
// the fixed TronGrid mainnet endpoint. Caller controls HTTP transport for tests.
// A single provider response is observational evidence, not independent confirmation.
func CollectTronMainnetPermissions(ctx context.Context, client *http.Client, address string) (TronAccountObservation, error) {
	const endpoint = "https://api.trongrid.io/wallet/getaccount"
	canonical := canonicalTronPermissionSigner(address)
	if canonical == "" {
		return TronAccountObservation{}, errors.New("invalid tron account address")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	body, err := json.Marshal(struct {
		Address string `json:"address"`
		Visible bool   `json:"visible"`
	}{Address: canonical, Visible: false})
	if err != nil {
		return TronAccountObservation{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return TronAccountObservation{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return TronAccountObservation{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TronAccountObservation{}, fmt.Errorf("tron getaccount HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return TronAccountObservation{}, err
	}
	if len(raw) > 1<<20 {
		return TronAccountObservation{}, errors.New("tron account response exceeds 1 MiB")
	}
	var account struct {
		Address     string `json:"address"`
		BlockNumber int64  `json:"block_number"`
	}
	if err := json.Unmarshal(raw, &account); err != nil {
		return TronAccountObservation{}, err
	}
	if canonicalTronPermissionSigner(account.Address) != canonical {
		return TronAccountObservation{}, errors.New("tron provider account address mismatch or missing")
	}
	if account.BlockNumber <= 0 {
		return TronAccountObservation{}, errors.New("tron account response lacks a block number")
	}
	permissions, err := ParseTronAccountPermissions(raw)
	if err != nil {
		return TronAccountObservation{}, err
	}
	digest := sha256.Sum256(raw)
	return TronAccountObservation{
		Network:           "tron-mainnet",
		Address:           canonical,
		Provider:          endpoint,
		RawResponseSHA256: hex.EncodeToString(digest[:]),
		BlockNumber:      account.BlockNumber,
		Permissions:       permissions,
	}, nil
}

// EvidenceRows exposes collected Tron permissions as observed-only rows. A
// positive chain height is mandatory: unanchored RPC responses are not evidence.
func (observation TronAccountObservation) EvidenceRows(observedAt time.Time) ([]EvidenceRow, error) {
	if observation.Network != "tron-mainnet" || observation.BlockNumber <= 0 ||
		canonicalTronPermissionSigner(observation.Address) != observation.Address ||
		observation.Provider == "" || len(observation.RawResponseSHA256) != 64 ||
		observedAt.IsZero() || len(observation.Permissions) == 0 {
		return nil, errors.New("incomplete tron observation provenance")
	}
	rows := make([]EvidenceRow, 0, len(observation.Permissions))
	for _, permission := range observation.Permissions {
		rows = append(rows, EvidenceRow{
			ID:        "tron-permission-" + permission.ID,
			SubjectID: observation.Address,
			Status:    "observed",
			Source:    observation.Provider,
			ObservedAt: observedAt,
			Height:    strconv.FormatInt(observation.BlockNumber, 10),
			RawHash:   observation.RawResponseSHA256,
			Attributes: map[string]any{
				"permission_id": permission.ID,
				"permission_type": permission.Type,
				"threshold": permission.Threshold,
				"total_weight": permission.TotalWeight,
				"threshold_reachable": permission.ThresholdReachable,
			},
		})
	}
	subject := SubjectIdentity{Network: observation.Network, IdentityKind: "account", IdentityValue: observation.Address, NativeID: observation.Address}
	if err := ValidateEvidence(subject, rows); err != nil {
		return nil, err
	}
	return rows, nil
}
