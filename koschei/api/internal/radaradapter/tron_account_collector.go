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
	"os"
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
	Permissions       []TronPermissionFinding
}

// CollectTronMainnetPermissions performs a read-only getaccount request against
// the fixed TronGrid mainnet endpoint. Caller controls HTTP transport for tests.
// A single provider response is observational evidence, not independent confirmation.
func CollectTronMainnetPermissions(ctx context.Context, client *http.Client, address string) (TronAccountObservation, error) {
	const endpoint = "https://api.trongrid.io/wallet/getaccount"
	const maxResponseBytes = 1048576
	canonical := canonicalTronPermissionSigner(address)
	if canonical == "" {
		return TronAccountObservation{}, errors.New("invalid tron account address")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	// Never allow a provider redirect to turn a read-only account query into
	// an unreviewed request to another host.
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if bounded.Timeout == 0 || bounded.Timeout > 8*time.Second {
		bounded.Timeout = 8 * time.Second
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
	if apiKey := os.Getenv("TRONGRID_API_KEY"); apiKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", apiKey)
	}
	resp, err := bounded.Do(req)
	if err != nil {
		return TronAccountObservation{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TronAccountObservation{}, fmt.Errorf("tron getaccount HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes + 1))
	if err != nil {
		return TronAccountObservation{}, err
	}
	if len(raw) > maxResponseBytes {
		return TronAccountObservation{}, errors.New("tron account response exceeds 1 MiB")
	}
	var account struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(raw, &account); err != nil {
		return TronAccountObservation{}, err
	}
	if canonicalTronPermissionSigner(account.Address) != canonical {
		return TronAccountObservation{}, errors.New("tron provider account address mismatch or missing")
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
		Permissions:       permissions,
	}, nil
}

// EvidenceRows exposes collected Tron permissions as observed-only rows. A
// positive, independently supplied chain height is mandatory. This height alone
// does not prove the account response came from that block.
func (observation TronAccountObservation) EvidenceRows(observedAt time.Time, verifiedBlockHeight int64) ([]EvidenceRow, error) {
	if observation.Network != "tron-mainnet" || verifiedBlockHeight <= 0 ||
		canonicalTronPermissionSigner(observation.Address) != observation.Address ||
		observation.Provider != "https://api.trongrid.io/wallet/getaccount" || len(observation.RawResponseSHA256) != 64 ||
		observedAt.IsZero() || len(observation.Permissions) == 0 {
		return nil, errors.New("incomplete tron observation provenance")
	}
	if _, err := hex.DecodeString(observation.RawResponseSHA256); err != nil {
		return nil, errors.New("invalid tron response digest encoding")
	}
	rows := make([]EvidenceRow, 0, len(observation.Permissions))
	for _, permission := range observation.Permissions {
		rows = append(rows, EvidenceRow{
			ID:         "tron-permission-" + permission.ID,
			SubjectID:  observation.Address,
			Status:     "observed",
			Source:     observation.Provider,
			ObservedAt: observedAt,
			Height:     strconv.FormatInt(verifiedBlockHeight, 10),
			RawHash:    observation.RawResponseSHA256,
			Attributes: map[string]any{
				"permission_id":                  permission.ID,
				"permission_type":                permission.Type,
				"threshold":                      permission.Threshold,
				"total_weight":                   permission.TotalWeight,
				"key_count":                      permission.KeyCount,
				"threshold_reachable":            permission.ThresholdReachable,
				"operations":                     permission.Operations,
				"account_state_height_verified": false,
			},
		})
	}
	subject := SubjectIdentity{Network: observation.Network, IdentityKind: "account", IdentityValue: observation.Address, NativeID: observation.Address}
	if err := ValidateEvidence(subject, rows); err != nil {
		return nil, err
	}
	return rows, nil
}
