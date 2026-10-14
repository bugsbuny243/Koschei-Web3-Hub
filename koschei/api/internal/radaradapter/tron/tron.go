// Package tron collects permission state, never seed ownership or intent.
package tron

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	r "koschei/api/internal/radaradapter"
	"math/big"
	"net/url"
	"strconv"
	"strings"
)

// Mainnet block zero ID from TRON TIP-474.
const MainnetGenesis = "00000000000000001ebf88508a03865c71d452e25f4d51194196a1d22b6653dc"

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func Address(s string) (string, error) {
	if len(s) == 42 && strings.HasPrefix(s, "41") {
		if _, err := hex.DecodeString(s); err == nil {
			return strings.ToLower(s), nil
		}
	}
	if len(s) != 34 || s[0] != 'T' {
		return "", errors.New("Tron address requires case-sensitive Base58Check")
	}
	n := new(big.Int)
	for _, c := range s {
		i := strings.IndexRune(alphabet, c)
		if i < 0 {
			return "", errors.New("invalid Base58")
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(i)))
	}
	b := n.Bytes()
	if len(b) != 25 || b[0] != 0x41 {
		return "", errors.New("invalid Tron address payload")
	}
	a := sha256.Sum256(b[:21])
	h := sha256.Sum256(a[:])
	if !bytes.Equal(b[21:], h[:4]) {
		return "", errors.New("invalid Tron checksum")
	}
	return hex.EncodeToString(b[:21]), nil
}

type Key struct {
	Address string `json:"address"`
	Weight  int64  `json:"weight"`
}
type PermissionType int

func (p *PermissionType) UnmarshalJSON(b []byte) error {
	var value int
	if len(b) > 0 && b[0] == '"' {
		var name string
		if err := json.Unmarshal(b, &name); err != nil {
			return err
		}
		switch name {
		case "Owner":
			value = 0
		case "Witness":
			value = 1
		case "Active":
			value = 2
		default:
			return errors.New("unknown permission type")
		}
	} else if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	if value < 0 || value > 2 {
		return errors.New("unknown permission type")
	}
	*p = PermissionType(value)
	return nil
}

type Permission struct {
	ID         int            `json:"id"`
	Type       PermissionType `json:"type"`
	Threshold  int64          `json:"threshold"`
	Operations string         `json:"operations"`
	Keys       []Key          `json:"keys"`
}
type Control struct {
	ID                int    `json:"id"`
	ExternalThreshold bool   `json:"external_threshold"`
	SelfThreshold     bool   `json:"self_threshold"`
	Satisfiable       bool   `json:"satisfiable"`
	PermissionUpdate  bool   `json:"permission_update"`
	Operations        string `json:"operations"`
}

func AnalyzePermission(p Permission, self string) (Control, error) {
	c := Control{ID: p.ID, Operations: p.Operations}
	if p.Threshold <= 0 || len(p.Keys) == 0 {
		return c, errors.New("missing or invalid permission threshold")
	}
	total, external, own := new(big.Int), new(big.Int), new(big.Int)
	seen := map[string]bool{}
	for _, k := range p.Keys {
		addr := strings.ToLower(k.Address)
		if strings.HasPrefix(k.Address, "T") {
			var e error
			addr, e = Address(k.Address)
			if e != nil {
				return c, e
			}
		}
		b, e := hex.DecodeString(addr)
		if e != nil || len(b) != 21 || b[0] != 0x41 || k.Weight <= 0 || seen[addr] {
			return c, errors.New("invalid or duplicate permission key")
		}
		seen[addr] = true
		w := big.NewInt(k.Weight)
		total.Add(total, w)
		if addr == self {
			own.Add(own, w)
		} else {
			external.Add(external, w)
		}
	}
	threshold := big.NewInt(p.Threshold)
	c.Satisfiable = total.Cmp(threshold) >= 0
	c.ExternalThreshold = external.Cmp(threshold) >= 0
	c.SelfThreshold = own.Cmp(threshold) >= 0
	c.PermissionUpdate = p.ID == 0
	if p.ID >= 2 {
		b, e := hex.DecodeString(p.Operations)
		if e != nil || len(b) != 32 {
			return c, errors.New("active permission bitmap missing")
		}
		c.PermissionUpdate = b[46/8]&(1<<uint(46%8)) != 0
	}
	return c, nil
}

type Adapter struct {
	target     r.Target
	transport  r.Transport
	subject    r.SubjectIdentity
	hexAddress string
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	a, e := Address(t.Raw)
	if e != nil {
		return nil, e
	}
	return &Adapter{t, tr, r.SubjectIdentity{Network: t.Network, NativeID: a}, a}, nil
}
func (a *Adapter) Supports(t r.Target) bool {
	return t.Network == "tron-mainnet" && (t.Kind == "address" || t.Kind == "wallet" || t.Kind == "token" || t.Kind == "contract")
}
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "tron-account-model", Network: a.target.Network, Family: "tron", TargetClasses: []string{"address", "wallet", "token", "contract"}, Sources: []string{"walletsolidity/getaccount", "wallet/getcontract", "TronGrid indexed history"}, ArchivalRequirements: []string{"Deep historical traces require an archive/indexer; bounded transfers are not holder balances."}, Fixture: "tron/testdata/account.json", EvidenceOnly: true}
}

type account struct {
	Address  string          `json:"address"`
	Owner    *Permission     `json:"owner_permission"`
	Active   []Permission    `json:"active_permission"`
	Witness  *Permission     `json:"witness_permission"`
	Resource json.RawMessage `json:"account_resource"`
	Frozen   json.RawMessage `json:"frozenV2"`
}

func (a *Adapter) post(ctx context.Context, p, path string, body any) (r.Response, error) {
	return a.transport.Do(ctx, r.Request{Network: a.target.Network, Provider: p, Method: "POST", Path: path, Body: body})
}
func (a *Adapter) snapshot(ctx context.Context, p string) (r.Response, account, string, string, error) {
	fail := func(e error) (r.Response, account, string, string, error) { return r.Response{}, account{}, "", "", e }
	gen, e := a.post(ctx, p, "/wallet/getblockbynum", map[string]any{"num": 0})
	if e != nil {
		return fail(e)
	}
	var genesis struct {
		ID string `json:"blockID"`
	}
	if r.Decode(gen, &genesis) != nil || genesis.ID != MainnetGenesis {
		return fail(errors.New("genesis identity missing"))
	}
	head, e := a.post(ctx, p, "/walletsolidity/getnowblock", map[string]any{})
	if e != nil {
		return fail(e)
	}
	var block struct {
		ID     string `json:"blockID"`
		Header struct {
			Raw struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if r.Decode(head, &block) != nil || block.ID == "" || block.Header.Raw.Number <= 0 {
		return fail(errors.New("confirmed head missing"))
	}
	res, e := a.post(ctx, p, "/walletsolidity/getaccount", map[string]any{"address": a.hexAddress, "visible": false})
	if e != nil {
		return fail(e)
	}
	var acc account
	if r.Decode(res, &acc) != nil || strings.ToLower(acc.Address) != a.hexAddress || acc.Owner == nil {
		return fail(errors.New("account or permission state missing"))
	}
	if acc.Owner.Type != 0 || acc.Owner.ID != 0 {
		return fail(errors.New("invalid owner permission"))
	}
	seen := map[int]bool{0: true}
	for _, p := range acc.Active {
		if p.Type != 2 || p.ID < 2 || p.ID > 9 || seen[p.ID] {
			return fail(errors.New("invalid active permission"))
		}
		seen[p.ID] = true
	}
	if acc.Witness != nil && (acc.Witness.ID != 1 || acc.Witness.Type != 1) {
		return fail(errors.New("invalid witness permission"))
	}
	return res, acc, genesis.ID, strconv.FormatInt(block.Header.Raw.Number, 10), nil
}
func (a *Adapter) Collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows := []r.EvidenceRow{}
	limits := []r.Limitation{{Code: "state_window", Reason: "Confirmed account state is read after the stated confirmed head, not a historical state proof. External control may be legitimate multisig; no private-key availability or malicious intent is inferred."}}
	provider := "primary"
	res, acc, gen, height, e := a.snapshot(ctx, provider)
	if e != nil {
		provider = "secondary"
		res, acc, gen, height, e = a.snapshot(ctx, provider)
	}
	if e != nil {
		return rows, append(limits, r.Limitation{Code: "permissions_unknown", Reason: "Both providers failed to return a bound account snapshot."}), e
	}
	a.subject.ChainIdentity = "genesis:" + gen
	other := "secondary"
	if provider == "secondary" {
		other = "primary"
	}
	second, acc2, gen2, _, e2 := a.snapshot(ctx, other)
	p1, _ := json.Marshal([]any{acc.Owner, acc.Active})
	p2, _ := json.Marshal([]any{acc2.Owner, acc2.Active})
	confirmed := e2 == nil && gen == gen2 && res.Host != second.Host && bytes.Equal(p1, p2)
	if !confirmed {
		limits = append(limits, r.Limitation{Code: "independent_confirmation_missing", Reason: "Permission snapshots were unavailable, changed or disagreed across provider hosts."})
	}
	perms := append([]Permission{*acc.Owner}, acc.Active...)
	if acc.Witness != nil {
		perms = append(perms, *acc.Witness)
	}
	validPermissions := true
	canUpdate := false
	selfUpdate := false
	for _, p := range perms {
		control, err := AnalyzePermission(p, a.hexAddress)
		if err != nil {
			validPermissions = false
			limits = append(limits, r.Limitation{Code: "malformed_permission", Reason: err.Error()})
			continue
		}
		if control.PermissionUpdate && control.Satisfiable {
			canUpdate = true
		}
		if control.PermissionUpdate && control.SelfThreshold {
			selfUpdate = true
		}
		row := r.NewRow(a.subject, "tron-permission-state", res, height, map[string]any{"permission": p, "control": control, "cross_provider_agreement": confirmed, "confirmation_response_sha256": r.Digest(second.Raw)})
		if control.ExternalThreshold && p.ID != 1 {
			row.Relation = "tron-external-permission-control"
			row.Attributes["meaning"] = "External keys meet the permission threshold; this is authority exposure, not proof of a scam."
			row.Attributes["critical"] = true
			if confirmed {
				row.Attributes["confirmation_hosts"] = []string{res.Host, second.Host}
				row.Status = "verified"
			}
		}
		rows = append(rows, row)
	}
	if validPermissions {
		rows = append(rows, r.NewRow(a.subject, "tron-permission-recovery", res, height, map[string]any{"configured_keys_can_update": canUpdate, "self_can_update": selfUpdate, "key_possession": "unknown"}))
	}
	if len(acc.Resource) > 0 {
		rows = append(rows, r.NewRow(a.subject, "tron-resource-state", res, height, map[string]any{"account_resource": acc.Resource, "frozen_v2": acc.Frozen}))
	}
	if t.Kind == "contract" || t.Kind == "token" {
		contract, err := a.post(ctx, provider, "/wallet/getcontract", map[string]any{"value": a.hexAddress})
		if err == nil {
			var c map[string]any
			if r.Decode(contract, &c) == nil && strings.EqualFold(fmt.Sprint(c["contract_address"]), a.hexAddress) {
				rows = append(rows, r.NewRow(a.subject, "tron-contract-control", contract, height, map[string]any{"contract": c}))
			}
		}
		for _, method := range []string{"totalSupply()", "decimals()"} {
			cr, ce := a.post(ctx, provider, "/walletsolidity/triggerconstantcontract", map[string]any{"owner_address": a.hexAddress, "contract_address": a.hexAddress, "function_selector": method, "visible": false})
			var output struct {
				Result struct {
					OK bool `json:"result"`
				} `json:"result"`
				Values []string `json:"constant_result"`
			}
			if ce == nil && r.Decode(cr, &output) == nil && output.Result.OK && len(output.Values) == 1 {
				if b, err := hex.DecodeString(output.Values[0]); err == nil && len(b) == 32 {
					rows = append(rows, r.NewRow(a.subject, "tron-trc20-"+strings.TrimSuffix(method, "()"), cr, height, map[string]any{"value": new(big.Int).SetBytes(b).String(), "account_state_height_verified": false}))
					continue
				}
			}
			limits = append(limits, r.Limitation{Code: "trc20_constant_unavailable", Reason: method + " could not be read."})
		}
		limits = append(limits, r.Limitation{Code: "trc20_holder_index_missing", Reason: "ABI/origin and constant calls do not prove effective mint authority or historical holder concentration."})
	}
	history, err := a.transport.Do(ctx, r.Request{Network: t.Network, Provider: "indexed", Method: "GET", Path: "/v1/accounts/" + url.PathEscape(a.hexAddress) + "/transactions?only_confirmed=true&limit=50"})
	if err == nil {
		var h struct {
			Data []json.RawMessage `json:"data"`
		}
		if r.Decode(history, &h) == nil {
			for _, tx := range h.Data {
				var item struct {
					ID    string `json:"txID"`
					Block int64  `json:"blockNumber"`
					Raw   struct {
						Contracts []struct {
							Type      string `json:"type"`
							Parameter struct {
								Value struct {
									Owner string `json:"owner_address"`
								} `json:"value"`
							} `json:"parameter"`
						} `json:"contract"`
					} `json:"raw_data"`
				}
				if json.Unmarshal(tx, &item) != nil {
					continue
				}
				for _, c := range item.Raw.Contracts {
					if c.Type == "AccountPermissionUpdateContract" && strings.EqualFold(c.Parameter.Value.Owner, a.hexAddress) && item.ID != "" && item.Block > 0 {
						row := r.NewRow(a.subject, "tron-permission-update", history, strconv.FormatInt(item.Block, 10), map[string]any{"transaction": json.RawMessage(tx)})
						row.Attributes["transaction_hash"] = item.ID
						rows = append(rows, row)
					}
				}
			}
		}
	}
	tokenRows, tokenLimits := a.tokenHistory(ctx, height)
	rows = append(rows, tokenRows...)
	limits = append(limits, tokenLimits...)
	limits = append(limits, r.Limitation{Code: "bounded_history", Reason: "At most 50 confirmed indexed transactions are inspected. Missing updates are not proof that permissions never changed; transfer samples are not holder concentration."})
	return rows, limits, nil
}
