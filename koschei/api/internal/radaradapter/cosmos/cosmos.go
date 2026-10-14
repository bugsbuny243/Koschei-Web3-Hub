package cosmos

import (
	"context"
	"encoding/json"
	"errors"
	r "koschei/api/internal/radaradapter"
	"net/url"
	"strconv"
	"strings"
)

type Chain struct{ ID, Prefix string }

var Chains = map[string]Chain{"cosmoshub-mainnet": {"cosmoshub-4", "cosmos"}, "osmosis-mainnet": {"osmosis-1", "osmo"}, "celestia-mainnet": {"celestia", "celestia"}, "injective-mainnet": {"injective-1", "inj"}, "sei-mainnet": {"pacific-1", "sei"}}

const chars = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func Address(s, prefix string) error {
	if s != strings.ToLower(s) || len(s) > 90 {
		return errors.New("noncanonical bech32 address")
	}
	i := strings.LastIndexByte(s, '1')
	if i < 1 || s[:i] != prefix || len(s)-i < 7 {
		return errors.New("address prefix mismatch")
	}
	values := []byte{}
	for _, c := range prefix {
		values = append(values, byte(c)>>5)
	}
	values = append(values, 0)
	for _, c := range prefix {
		values = append(values, byte(c)&31)
	}
	for _, c := range s[i+1:] {
		v := strings.IndexRune(chars, c)
		if v < 0 {
			return errors.New("invalid bech32 alphabet")
		}
		values = append(values, byte(v))
	}
	check := uint32(1)
	g := []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, v := range values {
		top := check >> 25
		check = (check&0x1ffffff)<<5 ^ uint32(v)
		for k := 0; k < 5; k++ {
			if top>>k&1 != 0 {
				check ^= g[k]
			}
		}
	}
	if check != 1 {
		return errors.New("invalid bech32 checksum")
	}
	count := len(s) - i - 7
	bits := count * 5
	bytes := bits / 8
	if bytes != 20 && bytes != 32 {
		return errors.New("unsupported account length")
	}
	pad := bits % 8
	if pad >= 5 {
		return errors.New("invalid bech32 padding")
	}
	if pad > 0 && strings.IndexByte(chars, s[len(s)-7])&((1<<pad)-1) != 0 {
		return errors.New("nonzero bech32 padding")
	}
	return nil
}

type Adapter struct {
	target    r.Target
	subject   r.SubjectIdentity
	transport r.Transport
	provider  string
}

func New(t r.Target, tr r.Transport) (r.Adapter, error) {
	chain, ok := Chains[t.Network]
	if !ok {
		return nil, errors.New("unregistered Cosmos chain")
	}
	if t.Kind == "bridge" {
		p := strings.Split(t.Raw, "/")
		if len(p) != 2 || p[0] == "" || !strings.HasPrefix(p[1], "channel-") {
			return nil, errors.New("IBC target requires port/channel-N")
		}
		if _, e := strconv.ParseUint(strings.TrimPrefix(p[1], "channel-"), 10, 64); e != nil {
			return nil, e
		}
	} else if t.Kind != "governance" {
		if e := Address(t.Raw, chain.Prefix); e != nil {
			return nil, e
		}
	} else if t.Raw != "proposals" {
		return nil, errors.New("governance target must be proposals")
	}
	return &Adapter{t, r.SubjectIdentity{Network: t.Network, NativeID: t.Raw, ChainIdentity: "cosmos:" + chain.ID}, tr, "primary"}, nil
}
func (a *Adapter) Supports(t r.Target) bool {
	_, ok := Chains[t.Network]
	return ok && (t.Kind == "address" || t.Kind == "wallet" || t.Kind == "bridge" || t.Kind == "governance")
}
func (a *Adapter) Subject() r.SubjectIdentity { return r.CollectorIdentity(a.subject) }
func (a *Adapter) Capability() r.Capability {
	return r.Capability{ID: "cosmos-cometbft-model", Network: a.target.Network, Family: "cosmos", TargetClasses: []string{"address", "wallet", "bridge", "governance"}, Sources: []string{"Cosmos SDK REST authz/feegrant/gov", "IBC channel/client state"}, ArchivalRequirements: []string{"Past grants and historical IBC/consensus state need archival LCD/indexer."}, Fixture: "cosmos/cosmos_test.go", EvidenceOnly: true}
}
func (a *Adapter) get(ctx context.Context, path string) (r.Response, error) {
	return a.transport.Do(ctx, r.Request{Network: a.target.Network, Provider: a.provider, Method: "GET", Path: path})
}
func (a *Adapter) Collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows, limits, e := a.collect(ctx, t)
	if e != nil {
		a.provider = "secondary"
		return a.collect(ctx, t)
	}
	return rows, limits, nil
}
func (a *Adapter) collect(ctx context.Context, t r.Target) ([]r.EvidenceRow, []r.Limitation, error) {
	rows := []r.EvidenceRow{}
	limits := []r.Limitation{{Code: "bounded_lcd_snapshot", Reason: "Queries are bounded to 100 entries. They follow the observed head and are not an atomic historical proof; missing pages and expired grants do not establish zero exposure."}}
	head, e := a.get(ctx, "/cosmos/base/tendermint/v1beta1/blocks/latest")
	var b struct {
		Block struct {
			Header struct {
				ChainID string `json:"chain_id"`
				Height  string `json:"height"`
			} `json:"header"`
		} `json:"block"`
		SDKBlock struct {
			Header struct {
				ChainID string `json:"chain_id"`
				Height  string `json:"height"`
			} `json:"header"`
		} `json:"sdk_block"`
	}
	if e != nil || r.Decode(head, &b) != nil {
		return rows, limits, errors.New("chain head unavailable")
	}
	header := b.Block.Header
	if header.ChainID == "" {
		header = b.SDKBlock.Header
	}
	if header.ChainID != Chains[t.Network].ID || header.Height == "" {
		return rows, limits, errors.New("Cosmos chain identity mismatch")
	}
	paths := []struct{ relation, path, field string }{}
	switch t.Kind {
	case "address", "wallet":
		paths = append(paths, struct{ relation, path, field string }{"cosmos-authz-grants", "/cosmos/authz/v1beta1/grants/granter/" + url.PathEscape(t.Raw) + "?pagination.limit=100", "grants"}, struct{ relation, path, field string }{"cosmos-feegrant-allowances", "/cosmos/feegrant/v1beta1/issued/" + url.PathEscape(t.Raw) + "?pagination.limit=100", "allowances"})
	case "governance":
		paths = append(paths, struct{ relation, path, field string }{"cosmos-governance-proposals", "/cosmos/gov/v1/proposals?pagination.limit=100", "proposals"})
	case "bridge":
		p := strings.Split(t.Raw, "/")
		paths = append(paths, struct{ relation, path, field string }{"ibc-channel-state", "/ibc/core/channel/v1/channels/" + url.PathEscape(p[1]) + "/ports/" + url.PathEscape(p[0]), "channel"}, struct{ relation, path, field string }{"ibc-client-states", "/ibc/core/client/v1/client_states?pagination.limit=100", "client_states"})
	}
	for _, path := range paths {
		response, err := a.get(ctx, path.path)
		if err != nil {
			limits = append(limits, r.Limitation{Code: "module_unavailable", Reason: path.relation})
			continue
		}
		var obj map[string]json.RawMessage
		if r.Decode(response, &obj) != nil || len(obj[path.field]) == 0 {
			limits = append(limits, r.Limitation{Code: "module_field_missing", Reason: path.relation})
			continue
		}
		rows = append(rows, r.NewRow(a.subject, path.relation, response, header.Height, map[string]any{"state": obj[path.field], "pagination": obj["pagination"], "head_response_sha256": r.Digest(head.Raw)}))
	}
	if len(rows) == 0 {
		return rows, limits, errors.New("all Cosmos evidence sources unavailable")
	}
	return rows, limits, nil
}
