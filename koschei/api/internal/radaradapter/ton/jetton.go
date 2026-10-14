package ton

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	tonaddress "github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tvm/cell"
	r "koschei/api/internal/radaradapter"
	"math/big"
	"strings"
)

// Explicit owner|master|wallet input is required: symbol metadata is not identity.
func jettonTarget(raw string) ([]string, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return nil, errors.New("jetton_wallet requires owner|master|wallet")
	}
	for i, p := range parts {
		a, e := Address(p)
		if e != nil || a.Testnet {
			return nil, errors.New("invalid mainnet jetton identity")
		}
		parts[i] = a.Raw
	}
	return parts, nil
}
func ownerSlice(raw string) (string, error) {
	address, e := tonaddress.ParseRawAddr(raw)
	if e != nil {
		return "", e
	}
	b := cell.BeginCell()
	if e = b.StoreAddr(address); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(b.EndCell().ToBOC()), nil
}
func stackAddress(raw json.RawMessage) (string, error) {
	var entry []json.RawMessage
	if json.Unmarshal(raw, &entry) != nil || len(entry) != 2 {
		return "", errors.New("invalid address stack")
	}
	var kind string
	_ = json.Unmarshal(entry[0], &kind)
	if kind != "cell" && kind != "slice" {
		return "", errors.New("invalid address stack type")
	}
	var body struct {
		Bytes string `json:"bytes"`
	}
	if json.Unmarshal(entry[1], &body) != nil {
		return "", errors.New("missing cell")
	}
	bytes, e := base64.StdEncoding.DecodeString(body.Bytes)
	if e != nil || len(bytes) > 4096 {
		return "", errors.New("invalid address cell")
	}
	c, e := cell.FromBOC(bytes)
	if e != nil {
		return "", e
	}
	addr, e := c.BeginParse().LoadAddr()
	if e != nil || addr == nil {
		return "", errors.New("invalid cell address")
	}
	parsed, e := Address(addr.StringRaw())
	if e != nil {
		return "", e
	}
	return parsed.Raw, nil
}
func (a *Adapter) getMethod(ctx context.Context, address, method string, stack any) (r.Response, []json.RawMessage, error) {
	res, e := a.transport.Do(ctx, r.Request{Network: a.target.Network, Provider: a.provider, Method: "POST", Path: "/api/v2/runGetMethod", Body: map[string]any{"address": address, "method": method, "stack": stack}})
	if e != nil {
		return res, nil, e
	}
	var payload struct {
		OK     bool `json:"ok"`
		Result struct {
			ExitCode *int              `json:"exit_code"`
			Stack    []json.RawMessage `json:"stack"`
		} `json:"result"`
	}
	if r.Decode(res, &payload) != nil || !payload.OK || payload.Result.ExitCode == nil || *payload.Result.ExitCode != 0 {
		return res, nil, errors.New("jetton get method failed")
	}
	return res, payload.Result.Stack, nil
}
func (a *Adapter) jettonWallet(ctx context.Context, limits []r.Limitation, height string) ([]r.EvidenceRow, []r.Limitation, error) {
	parts, e := jettonTarget(a.target.Raw)
	if e != nil {
		return nil, limits, e
	}
	owner, master, wallet := parts[0], parts[1], parts[2]
	registry, ok := a.transport.(interface{ CanonicalJetton(string) bool })
	if !ok || !registry.CanonicalJetton(master) {
		return nil, append(limits, r.Limitation{Code: "canonical_master_missing", Reason: "Operator-pinned master is required before attributing a jetton balance."}), errors.New("canonical master unavailable")
	}
	input, e := ownerSlice(owner)
	if e != nil {
		return nil, limits, e
	}
	derived, stack, e := a.getMethod(ctx, master, "get_wallet_address", [][]any{{"tvm.Slice", input}})
	if e != nil || len(stack) != 1 {
		return nil, limits, errors.New("jetton wallet derivation unavailable")
	}
	address, e := stackAddress(stack[0])
	if e != nil || address != wallet {
		return nil, append(limits, r.Limitation{Code: "jetton_wallet_mismatch", Reason: "Claimed wallet differs from master-derived address."}), errors.New("jetton wallet mismatch")
	}
	data, stack, e := a.getMethod(ctx, wallet, "get_wallet_data", [][]any{})
	if e != nil || len(stack) != 4 {
		return nil, limits, errors.New("jetton wallet data unavailable")
	}
	actualOwner, e1 := stackAddress(stack[1])
	actualMaster, e2 := stackAddress(stack[2])
	if e1 != nil || e2 != nil || actualOwner != owner || actualMaster != master {
		return nil, append(limits, r.Limitation{Code: "jetton_wallet_identity_mismatch", Reason: "Wallet getter owner/master mismatch."}), errors.New("jetton wallet identity mismatch")
	}
	var amount []string
	if json.Unmarshal(stack[0], &amount) != nil || len(amount) != 2 || amount[0] != "num" {
		return nil, limits, errors.New("jetton balance missing")
	}
	balance, ok := new(big.Int).SetString(strings.TrimPrefix(amount[1], "0x"), 16)
	if !ok || balance.Sign() < 0 {
		return nil, limits, errors.New("invalid jetton balance")
	}
	facts := map[string]any{"owner": owner, "master": master, "wallet": wallet, "balance_raw": balance.String(), "canonical_master_pinned": true, "derivation_response_sha256": r.Digest(derived.Raw), "account_state_height_verified": false}
	limits = append(limits, r.Limitation{Code: "jetton_state_window", Reason: "Getter responses are observed sequentially; no atomic historical state proof or transfer settlement is claimed."})
	return []r.EvidenceRow{r.NewRow(a.subject, "ton-jetton-wallet-binding", data, height, facts)}, limits, nil
}

// CodeHash hashes the TON cell graph, not its BOC transport serialization.
func CodeHash(boc string) (string, error) {
	raw, e := base64.StdEncoding.DecodeString(boc)
	if e != nil {
		return "", e
	}
	c, e := cell.FromBOC(raw)
	if e != nil {
		return "", e
	}
	return hex.EncodeToString(c.Hash()), nil
}
