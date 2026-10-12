package radaradapter

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// TronHeightReader supplies a separately observed positive chain height.
// A height alone does not prove the getaccount response is state-anchored.
type TronHeightReader func(context.Context) (int64, error)

// TronWalletAdapter is an explicit read-only adapter for Tron account permissions.
// The universal dispatcher must still authorize the route before execution.
type TronWalletAdapter struct {
	Client     *http.Client
	ReadHeight TronHeightReader
	target     string
}

func NewTronWalletAdapter(client *http.Client, height TronHeightReader, address string) (*TronWalletAdapter, error) {
	canonical := canonicalTronPermissionSigner(address)
	if canonical == "" || height == nil {
		return nil, errors.New("tron adapter requires a valid account and height reader")
	}
	return &TronWalletAdapter{Client: client, ReadHeight: height, target: canonical}, nil
}

func (adapter *TronWalletAdapter) Supports(target Target) bool {
	return adapter != nil && target.Network == "tron-mainnet" &&
		(target.Kind == "wallet" || target.Kind == "account") &&
		canonicalTronPermissionSigner(target.Raw) == adapter.target
}

func (adapter *TronWalletAdapter) Subject() SubjectIdentity {
	if adapter == nil {
		return SubjectIdentity{}
	}
	return SubjectIdentity{Network: "tron-mainnet", IdentityKind: "account",
		IdentityValue: adapter.target, NativeID: adapter.target}
}

func (adapter *TronWalletAdapter) Capability() Capability {
	return Capability{Networks: []string{"tron-mainnet"}, TargetKinds: []string{"wallet", "account"}, DataSources: []string{"https://api.trongrid.io/wallet/getaccount"}, RequiresArchival: false, Fixture: "tron-account-permissions"}
}

func (adapter *TronWalletAdapter) Collect(ctx context.Context, target Target) ([]EvidenceRow, []Limitation, error) {
	if !adapter.Supports(target) {
		return nil, []Limitation{{Code: "UNSUPPORTED_TARGET", Reason: "Tron wallet adapter target mismatch."}}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	height, err := adapter.ReadHeight(ctx)
	if err != nil {
		return nil, nil, err
	}
	if height <= 0 {
		return nil, nil, errors.New("tron chain height unavailable")
	}
	observation, err := CollectTronMainnetPermissions(ctx, adapter.Client, adapter.target)
	if err != nil {
		return nil, nil, err
	}
	rows, err := observation.EvidenceRows(time.Now().UTC(), height)
	if err != nil {
		return nil, nil, err
	}
	return rows, []Limitation{{Code: "TRON_ACCOUNT_HEIGHT_UNANCHORED", Reason: "A separately observed chain height does not prove getaccount state at that height; evidence remains observed."}}, nil
}
